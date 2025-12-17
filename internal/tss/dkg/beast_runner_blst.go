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
	pending     map[int]wire.TSSDKG  // dealer -> share msg (waiting for commitments)

	acks map[int]map[int]struct{} // dealer -> set(fromIndex)

	done   bool
	result BeastDKGResult
}

func NewBeastDKGRunner(cfg BeastDKGConfig, tr p2p.TSSDKGTransport) (*BeastDKGRunner, error) {
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
		pending:     make(map[int]wire.TSSDKG),
		acks:        make(map[int]map[int]struct{}, cfg.N),
	}
	if r.epoch == 0 {
		r.epoch = 1
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

	// Broadcast our commitments and start periodic retries.
	r.broadcastCommitments(ctx)
	go r.retryLoop(ctx)
	return nil
}

func (r *BeastDKGRunner) retryLoop(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.mu.Lock()
			done := r.done
			r.mu.Unlock()
			if done {
				return
			}
			r.broadcastCommitments(ctx)
			r.broadcastMissingShares(ctx)
		}
	}
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
	for i := 1; i <= r.cfg.N; i++ {
		if i == r.cfg.Index {
			continue
		}
		r.mu.Lock()
		acked := r.acks[r.cfg.Index] != nil
		if acked {
			if _, ok := r.acks[r.cfg.Index][i]; ok {
				r.mu.Unlock()
				continue
			}
		}
		coeffs := r.coeffs
		r.mu.Unlock()
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done {
		return
	}
	if len(r.commitments) < r.cfg.N || len(r.shares) < r.cfg.N {
		return
	}

	// group pk = Σ commitments[idx][0]
	acc := new(blst.P1)
	for idx := 1; idx <= r.cfg.N; idx++ {
		com := r.commitments[idx]
		if len(com) == 0 {
			return
		}
		var aff blst.P1Affine
		if aff.Uncompress(com[0]) == nil {
			return
		}
		var p blst.P1
		p.FromAffine(&aff)
		acc.AddAssign(&p)
	}
	gpk := acc.ToAffine().Compress()

	// share scalar = Σ shares[dealer]
	sum := scalarFromInt(0)
	for idx := 1; idx <= r.cfg.N; idx++ {
		s := r.shares[idx]
		if s == nil {
			return
		}
		if _, ok := sum.AddAssign(s); !ok {
			return
		}
	}
	shareScalar := sum.Serialize()

	// Persist keyshare and mark done.
	_ = r.store.SaveKeyShare(ctx, KeyShare{Index: r.cfg.Index, PublicKey: gpk, PrivateKey: shareScalar})
	r.done = true
	r.result = BeastDKGResult{Index: r.cfg.Index, Threshold: r.cfg.Threshold, GroupPubKey: gpk, ShareScalar: shareScalar}
	r.persistLocked()

	logger.InfoJ("beast_dkg", map[string]any{"result": "ok", "index": r.cfg.Index, "threshold": r.cfg.Threshold})
	metrics.Inc("beast_dkg_total", map[string]string{"result": "ok"})
}

func (r *BeastDKGRunner) OnMessage(ctx context.Context, m wire.TSSDKG) {
	if m.SessionID != r.cfg.SessionID {
		return
	}
	if m.Epoch != r.epoch || m.Epoch == 0 {
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
	case "ack":
		r.onAck(m)
	case "complaint":
		if m.ToIndex == r.cfg.Index {
			// resend share to the complaining node on next retry tick
			r.broadcastMissingShares(ctx)
		}
	default:
	}
}

func (r *BeastDKGRunner) onCommitments(m wire.TSSDKG) {
	if len(m.Commitments) == 0 {
		return
	}
	for _, c := range m.Commitments {
		if len(c) != 48 {
			return
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.commitments[m.FromIndex]; ok {
		return
	}
	r.commitments[m.FromIndex] = clone2D(m.Commitments)
	r.persistLocked()
}

func (r *BeastDKGRunner) tryProcessPending(ctx context.Context, dealer int) {
	r.mu.Lock()
	msg, ok := r.pending[dealer]
	r.mu.Unlock()
	if !ok {
		return
	}
	r.onShare(ctx, msg)
}

func (r *BeastDKGRunner) onShare(ctx context.Context, m wire.TSSDKG) {
	r.mu.Lock()
	com := r.commitments[m.FromIndex]
	if len(com) == 0 {
		// wait for commitments
		r.pending[m.FromIndex] = m
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
	delete(r.pending, m.FromIndex)
	r.shares[m.FromIndex] = &sc
	r.persistLocked()
	r.mu.Unlock()

	r.broadcastAck(ctx, m.FromIndex)
}

func (r *BeastDKGRunner) broadcastAck(ctx context.Context, dealer int) {
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
	if m.ToIndex != r.cfg.Index {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket := r.acks[r.cfg.Index]
	if bucket == nil {
		bucket = map[int]struct{}{}
		r.acks[r.cfg.Index] = bucket
	}
	bucket[m.FromIndex] = struct{}{}
}
