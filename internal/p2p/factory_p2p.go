//go:build p2p

package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	p2phost "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
	"github.com/zmlAEQ/Aequa-network/internal/p2p/wire"
	"github.com/zmlAEQ/Aequa-network/internal/payload"
	"github.com/zmlAEQ/Aequa-network/pkg/logger"
	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// BuildTransport constructs a libp2p+gossipsub transport when 'p2p' tag enabled.
func BuildTransport(cfg NetConfig) (Transport, error) {
	t := &Libp2pTransport{cfg: cfg}
	return t, nil
}

// Libp2pTransport implements the Transport interface using libp2p + gossipsub.
type Libp2pTransport struct {
	cfg       NetConfig
	host      p2phost.Host
	ps        *pubsub.PubSub
	tq        *pubsub.Topic
	tt        *pubsub.Topic
	ttPriv    *pubsub.Topic
	tbShare   *pubsub.Topic
	subQ      *pubsub.Subscription
	subTx     *pubsub.Subscription
	subTxPriv *pubsub.Subscription
	subShare  *pubsub.Subscription
	onQBFT    func(qbft.Message)
	onTx      func(payload.Payload)
	onShare   func(wire.BeastShare)
}

func (t *Libp2pTransport) Start(ctx context.Context) error {
	if !t.cfg.Enable {
		return nil
	}
	opts := []libp2p.Option{}
	if len(t.cfg.Listen) > 0 {
		var addrs []ma.Multiaddr
		for _, s := range t.cfg.Listen {
			if strings.TrimSpace(s) == "" {
				continue
			}
			a, err := ma.NewMultiaddr(s)
			if err != nil {
				return err
			}
			addrs = append(addrs, a)
		}
		if len(addrs) > 0 {
			opts = append(opts, libp2p.ListenAddrs(addrs...))
		}
	}
	if t.cfg.NAT {
		opts = append(opts, libp2p.NATPortMap())
	}
	h, err := libp2p.New(opts...)
	if err != nil {
		return err
	}
	t.host = h
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return err
	}
	t.ps = ps
	if t.tq, err = ps.Join(wire.TopicQBFT); err != nil {
		return err
	}
	if t.tt, err = ps.Join(wire.TopicTx); err != nil {
		return err
	}
	if t.subQ, err = t.tq.Subscribe(); err != nil {
		return err
	}
	if t.subTx, err = t.tt.Subscribe(); err != nil {
		return err
	}
	if t.cfg.EnableBeast {
		if t.ttPriv, err = ps.Join(wire.TopicTxPrivate); err == nil {
			t.subTxPriv, _ = t.ttPriv.Subscribe()
		}
		if t.tbShare, err = ps.Join(wire.TopicBeastShare); err == nil {
			t.subShare, _ = t.tbShare.Subscribe()
		}
	}

	// connect bootnodes (best effort)
	for _, b := range t.cfg.Bootnodes {
		if strings.TrimSpace(b) == "" {
			continue
		}
		_ = connectOnce(ctx, h, b)
	}

	// Log self peer id and listen addrs for operators to copy into bootnodes.txt
	if h != nil {
		for _, a := range h.Addrs() {
			logger.InfoJ("p2p_addr", map[string]any{"self_id": h.ID().String(), "addr": a.String()})
		}
	}

	// receivers
	go t.loopQBFT(ctx)
	go t.loopTx(ctx)
	if t.cfg.EnableBeast && t.subTxPriv != nil {
		go t.loopTxPriv(ctx)
	}
	if t.cfg.EnableBeast && t.subShare != nil {
		go t.loopBeastShare(ctx)
	}
	logger.InfoJ("p2p_start", map[string]any{"result": "ok"})
	return nil
}

func (t *Libp2pTransport) Stop(ctx context.Context) error {
	if t.subQ != nil {
		_ = t.subQ.Cancel()
	}
	if t.subTx != nil {
		_ = t.subTx.Cancel()
	}
	if t.subTxPriv != nil {
		_ = t.subTxPriv.Cancel()
	}
	if t.subShare != nil {
		_ = t.subShare.Cancel()
	}
	if t.tq != nil {
		_ = t.tq.Close()
	}
	if t.tt != nil {
		_ = t.tt.Close()
	}
	if t.ttPriv != nil {
		_ = t.ttPriv.Close()
	}
	if t.tbShare != nil {
		_ = t.tbShare.Close()
	}
	if t.host != nil {
		return t.host.Close()
	}
	return nil
}

func (t *Libp2pTransport) BroadcastQBFT(_ context.Context, msg qbft.Message) error {
	if t.tq == nil {
		return errors.New("p2p not started")
	}
	w := wire.FromInternal(msg)
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	if err := t.tq.Publish(context.Background(), b); err != nil {
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction": "tx", "result": "error"})
		return err
	}
	metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction": "tx", "result": "ok"})
	metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicQBFT, "direction": "tx"})
	return nil
}

func (t *Libp2pTransport) BroadcastTx(_ context.Context, pl payload.Payload) error {
	if t.tt == nil {
		return errors.New("p2p not started")
	}
	w, ok := wire.TxFromInternal(pl)
	if !ok {
		return nil
	}
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	topic := wire.TopicTx
	tt := t.tt
	if pl.Type() == wire.TypePrivateV1 && t.ttPriv != nil {
		topic = wire.TopicTxPrivate
		tt = t.ttPriv
	}
	if err := tt.Publish(context.Background(), b); err != nil {
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": topic, "direction": "tx", "result": "error"})
		return err
	}
	metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": topic, "direction": "tx", "result": "ok"})
	metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": topic, "direction": "tx"})
	return nil
}

func (t *Libp2pTransport) OnQBFT(fn func(qbft.Message))          { t.onQBFT = fn }
func (t *Libp2pTransport) OnTx(fn func(payload.Payload))         { t.onTx = fn }
func (t *Libp2pTransport) OnBeastShare(fn func(wire.BeastShare)) { t.onShare = fn }

func (t *Libp2pTransport) BroadcastBeastShare(_ context.Context, msg wire.BeastShare) error {
	if t.tbShare == nil {
		return errors.New("p2p not started")
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := t.tbShare.Publish(context.Background(), b); err != nil {
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicBeastShare, "direction": "tx", "result": "error"})
		return err
	}
	metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicBeastShare, "direction": "tx", "result": "ok"})
	metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicBeastShare, "direction": "tx"})
	return nil
}

func (t *Libp2pTransport) loopQBFT(ctx context.Context) {
	for {
		m, err := t.subQ.Next(ctx)
		if err != nil {
			return
		}
		b := m.Data
		var w wire.QBFT
		if err := json.Unmarshal(b, &w); err != nil {
			metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction": "rx", "result": "decode_error"})
			continue
		}
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction": "rx", "result": "ok"})
		metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicQBFT, "direction": "rx"})
		if t.onQBFT != nil {
			t.onQBFT(w.ToInternal())
		}
	}
}

func (t *Libp2pTransport) loopTx(ctx context.Context) {
	for {
		m, err := t.subTx.Next(ctx)
		if err != nil {
			return
		}
		b := m.Data
		var w wire.TxEnvelope
		if err := json.Unmarshal(b, &w); err != nil {
			metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTx, "direction": "rx", "result": "decode_error"})
			continue
		}
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTx, "direction": "rx", "result": "ok"})
		metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicTx, "direction": "rx"})
		if t.onTx != nil {
			if pl := w.ToInternal(); pl != nil {
				t.onTx(pl)
			}
		}
	}
}

func (t *Libp2pTransport) loopTxPriv(ctx context.Context) {
	for {
		m, err := t.subTxPriv.Next(ctx)
		if err != nil {
			return
		}
		b := m.Data
		var w wire.TxEnvelope
		if err := json.Unmarshal(b, &w); err != nil {
			metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTxPrivate, "direction": "rx", "result": "decode_error"})
			continue
		}
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTxPrivate, "direction": "rx", "result": "ok"})
		metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicTxPrivate, "direction": "rx"})
		if t.onTx != nil {
			if pl := w.ToInternal(); pl != nil {
				t.onTx(pl)
			}
		}
	}
}

func (t *Libp2pTransport) loopBeastShare(ctx context.Context) {
	for {
		m, err := t.subShare.Next(ctx)
		if err != nil {
			return
		}
		b := m.Data
		var w wire.BeastShare
		if err := json.Unmarshal(b, &w); err != nil {
			metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicBeastShare, "direction": "rx", "result": "decode_error"})
			continue
		}
		metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicBeastShare, "direction": "rx", "result": "ok"})
		metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicBeastShare, "direction": "rx"})
		if t.onShare != nil {
			t.onShare(w)
		}
	}
}

func connectOnce(ctx context.Context, h p2phost.Host, addr string) error {
	maAddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return err
	}
	info, err := peer.AddrInfoFromP2pAddr(maAddr)
	if err != nil {
		return err
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return h.Connect(ctx2, *info)
}
