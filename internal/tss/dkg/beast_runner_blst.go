//go:build blst

package dkg

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/zmlAEQ/Aequa-network/internal/p2p"
	"github.com/zmlAEQ/Aequa-network/internal/p2p/wire"
	"github.com/zmlAEQ/Aequa-network/pkg/logger"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"

	blst "github.com/supranational/blst/bindings/go"
)

var (
	ErrDKGNotReady     = errors.New("dkg not ready")
	ErrDKGInvalidMsg   = errors.New("invalid dkg message")
	ErrDKGCrypto       = errors.New("dkg crypto error")
	ErrDKGUnauthorized = errors.New("dkg unauthorized")
)

type BeastDKGResult struct {
	Index      int
	Threshold  int
	GroupPubKey []byte
	ShareScalar []byte
}

type BeastDKGRunnerOpt func(*BeastDKGRunner)

func WithRetryInterval(d time.Duration) BeastDKGRunnerOpt {
	return func(r *BeastDKGRunner) {
		if d > 0 {
			r.retryInterval = d
		}
	}
}

func WithEpochTimeout(d time.Duration) BeastDKGRunnerOpt {
	return func(r *BeastDKGRunner) {
		if d > 0 {
			r.epochTimeout = d
		}
	}
}

// BeastDKGRunner runs a minimal Feldman DKG over an authenticated gossip channel.
// It is designed for "silent setup": run once to derive the committee master key,
// then reuse the per-node share for per-height BEAST decrypt shares.
type BeastDKGRunner struct {
	cfg   BeastDKGConfig
	tr    p2p.TSSDKGTransport
	store *KeyStore
	sess  *BeastSessionStore

	mu sync.Mutex

	epoch uint64

	coeffs          []*blst.Scalar
	selfCommitments [][]byte

	commitments map[int][][]byte
	shares      map[int]*blst.Scalar // dealer -> share for self
	pendingShare map[int]wire.TSSDKG // dealer -> share msg (waiting for commitments)
	pendingOpen  map[int]wire.TSSDKG // dealer -> open-share msg (waiting for commitments)

	acks       map[int]map[int]struct{} // dealer -> set(fromIndex) (ack sender indices)
	complaints map[int]map[int]struct{} // dealer -> set(complainant indices)
	badDealers map[int]struct{}         // disqualified dealers by public evidence

	done   bool
	finalizing bool
	result BeastDKGResult

	retryInterval time.Duration
	epochTimeout  time.Duration
	epochStart    time.Time
}

func NewBeastDKGRunner(cfg BeastDKGConfig, tr p2p.TSSDKGTransport, opts ...BeastDKGRunnerOpt) (*BeastDKGRunner, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if tr == nil {
		return nil, errors.New("nil transport")
	}
	ksPath := cfg.KeySharePath
	if ksPath == "" {
		ksPath = "tss_keyshare.dat"
	}
	var sess *BeastSessionStore
	if cfg.SessionDir != "" {
		sess = NewBeastSessionStore(cfg.SessionDir)
	}
	r := &BeastDKGRunner{
		cfg:         cfg,
		tr:          tr,
		store:       NewKeyStoreFromEnv(ksPath),
		sess:        sess,
		epoch:       cfg.Epoch,
		commitments: make(map[int][][]byte, cfg.N),
		shares:      make(map[int]*blst.Scalar, cfg.N),
		pendingShare: make(map[int]wire.TSSDKG),
		pendingOpen:  make(map[int]wire.TSSDKG),
		acks:         make(map[int]map[int]struct{}, cfg.N),
		complaints:   make(map[int]map[int]struct{}, cfg.N),
		badDealers:   make(map[int]struct{}, cfg.N),
		retryInterval: 2 * time.Second,
		epochTimeout:  60 * time.Second,
		epochStart:    time.Now(),
	}
	if r.epoch == 0 {
		r.epoch = 1
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r, nil
}

func (r *BeastDKGRunner) Result() (BeastDKGResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.done {
		return BeastDKGResult{}, false
	}
	return r.result, true
}

func (r *BeastDKGRunner) Start(ctx context.Context) error {
	// If already finalized, skip.
	if ks, err := r.store.LoadKeyShare(ctx); err == nil && len(ks.PrivateKey) == 32 {
		r.mu.Lock()
		r.done = true
		r.result = BeastDKGResult{Index: r.cfg.Index, Threshold: r.cfg.Threshold, GroupPubKey: ks.PublicKey, ShareScalar: ks.PrivateKey}
		r.mu.Unlock()
		logger.InfoJ("beast_dkg", map[string]any{"result": "skip", "reason": "keyshare_exists"})
		metrics.Inc("beast_dkg_total", map[string]string{"result": "skip"})
		return nil
	}

	// Load session state if enabled.
	if r.sess != nil {
		if st, err := r.sess.Load(r.cfg.SessionID); err == nil {
			if err := r.restore(st); err == nil {
				logger.InfoJ("beast_dkg", map[string]any{"result": "resume_ok"})
			}
		}
	}

	// Install transport handler.
	r.tr.OnTSSDKG(func(m wire.TSSDKG) { r.OnMessage(ctx, m) })

	// Initialize local polynomial (if not resumed).
	if err := r.ensureLocalPoly(); err != nil {
		return err
	}
	if r.epochStart.IsZero() {
		r.epochStart = time.Now()
	}

	// Broadcast our commitments and start periodic retries.
	r.broadcastCommitments(ctx)
	go r.retryLoop(ctx)
	return nil
}

func (r *BeastDKGRunner) retryLoop(ctx context.Context) {
	t := time.NewTicker(r.retryInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.mu.Lock()
			done := r.done
			start := r.epochStart
			timeout := r.epochTimeout
			epoch := r.epoch
			r.mu.Unlock()
			if done {
				return
			}
			if timeout > 0 && !start.IsZero() && time.Since(start) > timeout {
				r.bumpEpoch(ctx, epoch+1, "timeout")
				continue
			}
			r.broadcastCommitments(ctx)
			r.broadcastMissingShares(ctx)
		}
	}
}

func (r *BeastDKGRunner) resetForEpochLocked(epoch uint64) {
	r.epoch = epoch
	r.coeffs = nil
	r.selfCommitments = nil
	r.commitments = make(map[int][][]byte, r.cfg.N)
	r.shares = make(map[int]*blst.Scalar, r.cfg.N)
	r.pendingShare = make(map[int]wire.TSSDKG)
	r.pendingOpen = make(map[int]wire.TSSDKG)
	r.acks = make(map[int]map[int]struct{}, r.cfg.N)
	r.complaints = make(map[int]map[int]struct{}, r.cfg.N)
	r.badDealers = make(map[int]struct{}, r.cfg.N)
	r.done = false
	r.finalizing = false
	r.result = BeastDKGResult{}
	r.epochStart = time.Now()
}

func (r *BeastDKGRunner) bumpEpoch(ctx context.Context, epoch uint64, reason string) {
	if epoch == 0 {
		return
	}
	r.mu.Lock()
	if r.done || epoch <= r.epoch {
		r.mu.Unlock()
		return
	}
	r.resetForEpochLocked(epoch)
	r.persistLocked()
	r.mu.Unlock()

	logger.InfoJ("beast_dkg", map[string]any{"result": "epoch_bump", "epoch": epoch, "reason": reason})
	metrics.Inc("beast_dkg_total", map[string]string{"result": "epoch_bump"})
	_ = r.ensureLocalPoly()
	r.broadcastCommitments(ctx)
}

func (r *BeastDKGRunner) ensureLocalPoly() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.coeffs) > 0 && len(r.selfCommitments) > 0 {
		return nil
	}
	if len(r.cfg.SigPriv) != ed25519.PrivateKeySize {
		return errors.New("invalid sig_priv")
	}
	if len(r.cfg.EncPriv) != 32 {
		return errors.New("invalid enc_priv")
	}
	coeffs := make([]*blst.Scalar, 0, r.cfg.Threshold)
	for i := 0; i < r.cfg.Threshold; i++ {
		s, err := randScalar(rand.Reader)
		if err != nil {
			return err
		}
		coeffs = append(coeffs, s)
	}
	com, err := commitmentsFromPoly(coeffs)
	if err != nil {
		return err
	}
	selfShare, err := evalPolyAt(coeffs, r.cfg.Index)
	if err != nil {
		return err
	}
	r.coeffs = coeffs
	r.selfCommitments = com
	r.commitments[r.cfg.Index] = com
	// Include self dealer share so final aggregation can complete without network.
	r.shares[r.cfg.Index] = selfShare
	r.epochStart = time.Now()
	r.persistLocked()
	return nil
}

func (r *BeastDKGRunner) restore(st beastSessionState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st.Epoch == 0 {
		return ErrInvalidParams
	}
	r.epoch = st.Epoch
	if st.Done && len(st.ShareScalar) == 32 {
		r.done = true
		r.result = BeastDKGResult{Index: r.cfg.Index, Threshold: r.cfg.Threshold, GroupPubKey: st.GroupPubKey, ShareScalar: st.ShareScalar}
		return nil
	}
	if len(st.Coeffs) > 0 {
		coeffs := make([]*blst.Scalar, 0, len(st.Coeffs))
		for _, b := range st.Coeffs {
			var sc blst.Scalar
			if sc.Deserialize(b) == nil {
				return ErrInvalidShare
			}
			coeffs = append(coeffs, &sc)
		}
		r.coeffs = coeffs
	}
	if len(st.SelfCommitments) > 0 {
		r.selfCommitments = st.SelfCommitments
		r.commitments[r.cfg.Index] = st.SelfCommitments
	}
	if st.Commitments != nil {
		for idx, com := range st.Commitments {
			r.commitments[idx] = com
		}
	}
	if st.Shares != nil {
		for idx, b := range st.Shares {
			var sc blst.Scalar
			if sc.Deserialize(b) == nil {
				continue
			}
			r.shares[idx] = &sc
		}
	}
	if st.Acks != nil {
		for dealer, froms := range st.Acks {
			bucket := r.acks[dealer]
			if bucket == nil {
				bucket = map[int]struct{}{}
				r.acks[dealer] = bucket
			}
			for _, from := range froms {
				bucket[from] = struct{}{}
			}
		}
	}
	if st.Complaints != nil {
		for dealer, froms := range st.Complaints {
			bucket := r.complaints[dealer]
			if bucket == nil {
				bucket = map[int]struct{}{}
				r.complaints[dealer] = bucket
			}
			for _, from := range froms {
				bucket[from] = struct{}{}
			}
		}
	}
	for _, d := range st.Disqualified {
		if d > 0 {
			r.badDealers[d] = struct{}{}
		}
	}
	// Ensure self share exists when we have local coefficients.
	if len(r.coeffs) > 0 {
		if _, ok := r.shares[r.cfg.Index]; !ok {
			if sc, err := evalPolyAt(r.coeffs, r.cfg.Index); err == nil {
				r.shares[r.cfg.Index] = sc
			}
		}
	}
	return nil
}

func (r *BeastDKGRunner) persistLocked() {
	if r.sess == nil {
		return
	}
	st := beastSessionState{
		Epoch:           r.epoch,
		Coeffs:          scalarsToBytes(r.coeffs),
		SelfCommitments: clone2D(r.selfCommitments),
		Commitments:     cloneCommitmentsMap(r.commitments),
		Shares:          cloneScalarMap(r.shares),
		Acks:            cloneIndexSetMap(r.acks),
		Complaints:      cloneIndexSetMap(r.complaints),
		Disqualified:    cloneIndexSet(r.badDealers),
		Done:            r.done,
		GroupPubKey:     append([]byte(nil), r.result.GroupPubKey...),
		ShareScalar:     append([]byte(nil), r.result.ShareScalar...),
	}
	_ = r.sess.Save(r.cfg.SessionID, st)
}

func scalarsToBytes(in []*blst.Scalar) [][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(in))
	for _, s := range in {
		if s == nil {
			continue
		}
		out = append(out, append([]byte(nil), s.Serialize()...))
	}
	return out
}

func clone2D(in [][]byte) [][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = append([]byte(nil), in[i]...)
	}
	return out
}

func cloneCommitmentsMap(in map[int][][]byte) map[int][][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int][][]byte, len(in))
	for k, v := range in {
		out[k] = clone2D(v)
	}
	return out
}

func cloneScalarMap(in map[int]*blst.Scalar) map[int][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int][]byte, len(in))
	for k, v := range in {
		if v == nil {
			continue
		}
		out[k] = append([]byte(nil), v.Serialize()...)
	}
	return out
}

func cloneIndexSetMap(in map[int]map[int]struct{}) map[int][]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int][]int, len(in))
	for k, set := range in {
		if len(set) == 0 {
			continue
		}
		arr := make([]int, 0, len(set))
		for v := range set {
			arr = append(arr, v)
		}
		out[k] = arr
	}
	return out
}

func cloneIndexSet(in map[int]struct{}) []int {
	if len(in) == 0 {
		return nil
	}
	out := make([]int, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	return out
}

func (r *BeastDKGRunner) member(index int) (BeastDKGMember, bool) {
	for _, m := range r.cfg.Committee {
		if m.Index == index {
			return m, true
		}
	}
	return BeastDKGMember{}, false
}

func (r *BeastDKGRunner) signMessage(m wire.TSSDKG) (wire.TSSDKG, error) {
	if len(r.cfg.SigPriv) != ed25519.PrivateKeySize {
		return wire.TSSDKG{}, ErrDKGCrypto
	}
	m.Sig = nil
	b, err := json.Marshal(m)
	if err != nil {
		return wire.TSSDKG{}, err
	}
	sig := ed25519.Sign(ed25519.PrivateKey(r.cfg.SigPriv), b)
	m.Sig = sig
	return m, nil
}

func (r *BeastDKGRunner) verifySig(m wire.TSSDKG) bool {
	mem, ok := r.member(m.FromIndex)
	if !ok || len(mem.SigPub) != ed25519.PublicKeySize {
		return false
	}
	sig := append([]byte(nil), m.Sig...)
	m.Sig = nil
	b, err := json.Marshal(m)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(mem.SigPub), b, sig)
}

func (r *BeastDKGRunner) deriveShareKey(fromIndex, toIndex int) ([]byte, error) {
	// Shared secret: X25519(priv(self), pub(peer)).
	peer := 0
	switch {
	case fromIndex == r.cfg.Index && toIndex != r.cfg.Index:
		peer = toIndex
	case toIndex == r.cfg.Index && fromIndex != r.cfg.Index:
		peer = fromIndex
	default:
		return nil, ErrDKGUnauthorized
	}
	mem, ok := r.member(peer)
	if !ok || len(mem.EncPub) != 32 {
		return nil, ErrDKGUnauthorized
	}
	priv, err := ecdh.X25519().NewPrivateKey(r.cfg.EncPriv)
	if err != nil {
		return nil, err
	}
	pub, err := ecdh.X25519().NewPublicKey(mem.EncPub)
	if err != nil {
		return nil, err
	}
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, err
	}
	// KDF: SHA256(dst || shared || session_id || epoch || from || to)
	h := sha256.New()
	_, _ = h.Write([]byte("EQS/BEAST/DKG/v1"))
	_, _ = h.Write(shared)
	_, _ = h.Write([]byte(r.cfg.SessionID))
	var buf [8 + 4 + 4]byte
	binary.BigEndian.PutUint64(buf[0:8], r.epoch)
	binary.BigEndian.PutUint32(buf[8:12], uint32(fromIndex))
	binary.BigEndian.PutUint32(buf[12:16], uint32(toIndex))
	_, _ = h.Write(buf[:])
	sum := h.Sum(nil)
	return sum[:], nil
}

func encryptShare(key []byte, pt []byte) (nonce []byte, ct []byte, err error) {
	if len(key) != 32 {
		return nil, nil, ErrDKGCrypto
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct = gcm.Seal(nil, nonce, pt, nil)
	return nonce, ct, nil
}

func decryptShare(key []byte, nonce []byte, ct []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrDKGCrypto
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrDKGInvalidMsg
	}
	return gcm.Open(nil, nonce, ct, nil)
}

func (r *BeastDKGRunner) broadcastCommitments(ctx context.Context) {
	r.mu.Lock()
	msg := wire.TSSDKG{
		SessionID:   r.cfg.SessionID,
		Epoch:       r.epoch,
		Type:        "commitments",
		FromIndex:   r.cfg.Index,
		Commitments: clone2D(r.selfCommitments),
	}
	r.mu.Unlock()
	signed, err := r.signMessage(msg)
	if err != nil {
		return
	}
	_ = r.tr.BroadcastTSSDKG(ctx, signed)
}

func (r *BeastDKGRunner) broadcastMissingShares(ctx context.Context) {
	r.mu.Lock()
	acks := r.acks[r.cfg.Index]
	coeffs := r.coeffs
	r.mu.Unlock()

	for i := 1; i <= r.cfg.N; i++ {
		if i == r.cfg.Index {
			continue
		}
		if acks != nil {
			if _, ok := acks[i]; ok {
				continue
			}
		}
		if len(coeffs) == 0 {
			continue
		}
		sh, err := evalPolyAt(coeffs, i)
		if err != nil {
			continue
		}
		key, err := r.deriveShareKey(r.cfg.Index, i)
		if err != nil {
			continue
		}
		nonce, ct, err := encryptShare(key, sh.Serialize())
		if err != nil {
			continue
		}
		msg := wire.TSSDKG{
			SessionID:  r.cfg.SessionID,
			Epoch:      r.epoch,
			Type:       "share",
			FromIndex:  r.cfg.Index,
			ToIndex:    i,
			Nonce:      nonce,
			Ciphertext: ct,
		}
		signed, err := r.signMessage(msg)
		if err != nil {
			continue
		}
		_ = r.tr.BroadcastTSSDKG(ctx, signed)
	}
}

func (r *BeastDKGRunner) maybeFinalize(ctx context.Context) {
	var bumpEpoch uint64
	var gpk []byte
	var shareScalar []byte
	var epoch uint64
	var idx int
	var k int

	r.mu.Lock()
	if r.done || r.finalizing {
		r.mu.Unlock()
		return
	}
	// If there aren't enough remaining dealers to reach threshold, reshare via epoch bump.
	if (r.cfg.N - len(r.badDealers)) < r.cfg.Threshold {
		bumpEpoch = r.epoch + 1
		r.mu.Unlock()
		r.bumpEpoch(ctx, bumpEpoch, "insufficient_qual")
		return
	}
	qual := make([]int, 0, r.cfg.N)
	for dealer := 1; dealer <= r.cfg.N; dealer++ {
		if _, bad := r.badDealers[dealer]; bad {
			continue
		}
		if len(r.commitments[dealer]) == 0 {
			r.mu.Unlock()
			return
		}
		if c := r.complaints[dealer]; len(c) > 0 {
			r.mu.Unlock()
			return
		}
		acks := r.acks[dealer]
		if len(acks) < r.cfg.N-1 {
			r.mu.Unlock()
			return
		}
		if r.shares[dealer] == nil {
			r.mu.Unlock()
			return
		}
		qual = append(qual, dealer)
	}
	if len(qual) < r.cfg.Threshold {
		r.mu.Unlock()
		return
	}

	// group pk = Σ commitments[dealer][0] for dealer in QUAL
	acc := new(blst.P1)
	for _, dealer := range qual {
		com := r.commitments[dealer]
		var aff blst.P1Affine
		if aff.Uncompress(com[0]) == nil {
			r.mu.Unlock()
			return
		}
		var p blst.P1
		p.FromAffine(&aff)
		acc.AddAssign(&p)
	}
	gpk = acc.ToAffine().Compress()

	// share scalar = Σ shares[dealer] for dealer in QUAL
	sum := scalarFromInt(0)
	for _, dealer := range qual {
		s := r.shares[dealer]
		if s == nil {
			r.mu.Unlock()
			return
		}
		if _, ok := sum.AddAssign(s); !ok {
			r.mu.Unlock()
			return
		}
	}
	shareScalar = sum.Serialize()

	// Finalize outside the lock to avoid blocking gossip handling on I/O.
	r.finalizing = true
	epoch = r.epoch
	idx = r.cfg.Index
	k = r.cfg.Threshold
	r.mu.Unlock()

	_ = r.store.SaveKeyShare(ctx, KeyShare{Index: idx, PublicKey: gpk, PrivateKey: shareScalar})

	r.mu.Lock()
	if r.done || r.epoch != epoch {
		r.finalizing = false
		r.mu.Unlock()
		return
	}
	r.done = true
	r.result = BeastDKGResult{Index: idx, Threshold: k, GroupPubKey: gpk, ShareScalar: shareScalar}
	r.persistLocked()
	r.mu.Unlock()

	logger.InfoJ("beast_dkg", map[string]any{"result": "ok", "index": idx, "threshold": k})
	metrics.Inc("beast_dkg_total", map[string]string{"result": "ok"})
}

func (r *BeastDKGRunner) OnMessage(ctx context.Context, m wire.TSSDKG) {
	if m.SessionID != r.cfg.SessionID {
		return
	}
	if m.Epoch == 0 {
		return
	}
	if m.FromIndex <= 0 || m.FromIndex > r.cfg.N {
		return
	}
	if m.FromIndex == r.cfg.Index {
		return
	}
	if m.Type == "" {
		return
	}
	if !r.verifySig(m) {
		metrics.Inc("beast_dkg_total", map[string]string{"result": "bad_sig"})
		return
	}
	r.mu.Lock()
	done := r.done
	curEpoch := r.epoch
	r.mu.Unlock()
	if done {
		return
	}
	if m.Epoch < curEpoch {
		return
	}
	if m.Epoch > curEpoch {
		r.bumpEpoch(ctx, m.Epoch, "remote")
	}
	r.mu.Lock()
	curEpoch = r.epoch
	r.mu.Unlock()
	if m.Epoch != curEpoch {
		return
	}

	switch m.Type {
	case "commitments":
		r.onCommitments(m)
		r.tryProcessPending(ctx, m.FromIndex)
		r.broadcastMissingShares(ctx)
		r.maybeFinalize(ctx)
	case "share":
		if m.ToIndex != r.cfg.Index {
			return
		}
		r.onShare(ctx, m)
		r.maybeFinalize(ctx)
	case "share_open":
		r.onShareOpen(ctx, m)
		r.maybeFinalize(ctx)
	case "ack":
		r.onAck(m)
		r.maybeFinalize(ctx)
	case "complaint":
		r.onComplaint(ctx, m)
		r.maybeFinalize(ctx)
	default:
	}
}

func (r *BeastDKGRunner) disqualifyDealerLocked(dealer int, reason string) {
	if dealer <= 0 {
		return
	}
	if _, ok := r.badDealers[dealer]; ok {
		return
	}
	r.badDealers[dealer] = struct{}{}
	delete(r.complaints, dealer)
	logger.InfoJ("beast_dkg", map[string]any{"result": "dealer_disqualified", "dealer": dealer, "reason": reason})
	metrics.Inc("beast_dkg_total", map[string]string{"result": "dealer_disqualified"})
}

func (r *BeastDKGRunner) onCommitments(m wire.TSSDKG) {
	if len(m.Commitments) == 0 {
		return
	}
	if len(m.Commitments) != r.cfg.Threshold {
		r.mu.Lock()
		r.disqualifyDealerLocked(m.FromIndex, "bad_commitments_len")
		r.persistLocked()
		r.mu.Unlock()
		return
	}
	for _, c := range m.Commitments {
		if len(c) != 48 {
			r.mu.Lock()
			r.disqualifyDealerLocked(m.FromIndex, "bad_commitments_size")
			r.persistLocked()
			r.mu.Unlock()
			return
		}
		var aff blst.P1Affine
		if aff.Uncompress(c) == nil {
			r.mu.Lock()
			r.disqualifyDealerLocked(m.FromIndex, "bad_commitments_point")
			r.persistLocked()
			r.mu.Unlock()
			return
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, bad := r.badDealers[m.FromIndex]; bad {
		return
	}
	if _, ok := r.commitments[m.FromIndex]; ok {
		return
	}
	r.commitments[m.FromIndex] = clone2D(m.Commitments)
	r.persistLocked()
}

func (r *BeastDKGRunner) tryProcessPending(ctx context.Context, dealer int) {
	r.mu.Lock()
	msg, ok := r.pendingShare[dealer]
	r.mu.Unlock()
	if !ok {
		// continue to open-share below
	} else {
		r.onShare(ctx, msg)
	}

	r.mu.Lock()
	open, ok2 := r.pendingOpen[dealer]
	r.mu.Unlock()
	if ok2 {
		r.onShareOpen(ctx, open)
	}
}

func (r *BeastDKGRunner) onShare(ctx context.Context, m wire.TSSDKG) {
	r.mu.Lock()
	if _, bad := r.badDealers[m.FromIndex]; bad {
		r.mu.Unlock()
		return
	}
	com := r.commitments[m.FromIndex]
	if len(com) == 0 {
		// wait for commitments
		r.pendingShare[m.FromIndex] = m
		r.mu.Unlock()
		return
	}
	if _, ok := r.shares[m.FromIndex]; ok {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	key, err := r.deriveShareKey(m.FromIndex, r.cfg.Index)
	if err != nil {
		return
	}
	pt, err := decryptShare(key, m.Nonce, m.Ciphertext)
	if err != nil || len(pt) != 32 {
		r.broadcastComplaint(ctx, m.FromIndex)
		return
	}
	var sc blst.Scalar
	if sc.Deserialize(pt) == nil {
		r.broadcastComplaint(ctx, m.FromIndex)
		return
	}
	ok, err := verifyFeldmanShare(&sc, r.cfg.Index, com)
	if err != nil || !ok {
		r.broadcastComplaint(ctx, m.FromIndex)
		return
	}

	r.mu.Lock()
	delete(r.pendingShare, m.FromIndex)
	r.shares[m.FromIndex] = &sc
	r.persistLocked()
	r.mu.Unlock()

	r.broadcastAck(ctx, m.FromIndex)
}

func (r *BeastDKGRunner) broadcastAck(ctx context.Context, dealer int) {
	r.mu.Lock()
	bucket := r.acks[dealer]
	if bucket == nil {
		bucket = map[int]struct{}{}
		r.acks[dealer] = bucket
	}
	bucket[r.cfg.Index] = struct{}{}
	if c := r.complaints[dealer]; c != nil {
		delete(c, r.cfg.Index)
		if len(c) == 0 {
			delete(r.complaints, dealer)
		}
	}
	r.persistLocked()
	r.mu.Unlock()

	msg := wire.TSSDKG{
		SessionID:  r.cfg.SessionID,
		Epoch:      r.epoch,
		Type:       "ack",
		FromIndex:  r.cfg.Index,
		ToIndex:    dealer,
	}
	signed, err := r.signMessage(msg)
	if err != nil {
		return
	}
	_ = r.tr.BroadcastTSSDKG(ctx, signed)
}

func (r *BeastDKGRunner) broadcastComplaint(ctx context.Context, dealer int) {
	r.mu.Lock()
	bucket := r.complaints[dealer]
	if bucket == nil {
		bucket = map[int]struct{}{}
		r.complaints[dealer] = bucket
	}
	bucket[r.cfg.Index] = struct{}{}
	r.persistLocked()
	r.mu.Unlock()

	msg := wire.TSSDKG{
		SessionID: r.cfg.SessionID,
		Epoch:     r.epoch,
		Type:      "complaint",
		FromIndex: r.cfg.Index,
		ToIndex:   dealer,
	}
	signed, err := r.signMessage(msg)
	if err != nil {
		return
	}
	_ = r.tr.BroadcastTSSDKG(ctx, signed)
	metrics.Inc("beast_dkg_total", map[string]string{"result": "complaint"})
}

func (r *BeastDKGRunner) onAck(m wire.TSSDKG) {
	r.mu.Lock()
	defer r.mu.Unlock()
	dealer := m.ToIndex
	if dealer <= 0 || dealer > r.cfg.N {
		return
	}
	if _, bad := r.badDealers[dealer]; bad {
		return
	}
	bucket := r.acks[dealer]
	if bucket == nil {
		bucket = map[int]struct{}{}
		r.acks[dealer] = bucket
	}
	bucket[m.FromIndex] = struct{}{}
	if c := r.complaints[dealer]; c != nil {
		delete(c, m.FromIndex)
		if len(c) == 0 {
			delete(r.complaints, dealer)
		}
	}
	r.persistLocked()
}

func (r *BeastDKGRunner) onComplaint(ctx context.Context, m wire.TSSDKG) {
	dealer := m.ToIndex
	if dealer <= 0 || dealer > r.cfg.N {
		return
	}
	if dealer == r.cfg.Index {
		// Resolve by opening the share to the complainant.
		r.broadcastShareOpen(ctx, m.FromIndex)
		return
	}
	r.mu.Lock()
	if _, bad := r.badDealers[dealer]; bad {
		r.mu.Unlock()
		return
	}
	bucket := r.complaints[dealer]
	if bucket == nil {
		bucket = map[int]struct{}{}
		r.complaints[dealer] = bucket
	}
	bucket[m.FromIndex] = struct{}{}
	r.persistLocked()
	r.mu.Unlock()
}

func (r *BeastDKGRunner) broadcastShareOpen(ctx context.Context, toIndex int) {
	if toIndex <= 0 || toIndex > r.cfg.N || toIndex == r.cfg.Index {
		return
	}
	r.mu.Lock()
	coeffs := r.coeffs
	r.mu.Unlock()
	if len(coeffs) == 0 {
		return
	}
	sh, err := evalPolyAt(coeffs, toIndex)
	if err != nil {
		return
	}
	msg := wire.TSSDKG{
		SessionID:  r.cfg.SessionID,
		Epoch:      r.epoch,
		Type:       "share_open",
		FromIndex:  r.cfg.Index,
		ToIndex:    toIndex,
		Share:      sh.Serialize(),
	}
	signed, err := r.signMessage(msg)
	if err != nil {
		return
	}
	_ = r.tr.BroadcastTSSDKG(ctx, signed)
}

func (r *BeastDKGRunner) onShareOpen(ctx context.Context, m wire.TSSDKG) {
	if m.ToIndex <= 0 || m.ToIndex > r.cfg.N {
		return
	}
	if len(m.Share) != 32 {
		r.mu.Lock()
		r.disqualifyDealerLocked(m.FromIndex, "bad_open_share_len")
		r.persistLocked()
		r.mu.Unlock()
		return
	}
	r.mu.Lock()
	if _, bad := r.badDealers[m.FromIndex]; bad {
		r.mu.Unlock()
		return
	}
	com := r.commitments[m.FromIndex]
	if len(com) == 0 {
		r.pendingOpen[m.FromIndex] = m
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	var sc blst.Scalar
	if sc.Deserialize(m.Share) == nil {
		r.mu.Lock()
		r.disqualifyDealerLocked(m.FromIndex, "bad_open_share_scalar")
		r.persistLocked()
		r.mu.Unlock()
		return
	}
	ok, err := verifyFeldmanShare(&sc, m.ToIndex, com)
	if err != nil || !ok {
		r.mu.Lock()
		r.disqualifyDealerLocked(m.FromIndex, "bad_open_share_verify")
		r.persistLocked()
		r.mu.Unlock()
		return
	}

	// Complaint resolved: clear complainant entry for this dealer.
	r.mu.Lock()
	delete(r.pendingOpen, m.FromIndex)
	if c := r.complaints[m.FromIndex]; c != nil {
		delete(c, m.ToIndex)
		if len(c) == 0 {
			delete(r.complaints, m.FromIndex)
		}
	}
	if m.ToIndex == r.cfg.Index {
		if _, exists := r.shares[m.FromIndex]; !exists {
			r.shares[m.FromIndex] = &sc
		}
	}
	r.persistLocked()
	r.mu.Unlock()

	if m.ToIndex == r.cfg.Index {
		r.broadcastAck(ctx, m.FromIndex)
	}
}
