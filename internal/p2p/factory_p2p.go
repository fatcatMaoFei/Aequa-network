//go:build p2p

package p2p

import (
    "context"
    "encoding/json"
    "errors"
    "strings"
    "time"

    libp2p "github.com/libp2p/go-libp2p"
    p2phost "github.com/libp2p/go-libp2p/core/host"
    peer "github.com/libp2p/go-libp2p/core/peer"
    pubsub "github.com/libp2p/go-libp2p-pubsub"
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
type Libp2pTransport struct{
    cfg    NetConfig
    host   p2phost.Host
    ps     *pubsub.PubSub
    tq     *pubsub.Topic
    tt     *pubsub.Topic
    subQ   *pubsub.Subscription
    subTx  *pubsub.Subscription
    onQBFT func(qbft.Message)
    onTx   func(payload.Payload)
}

func (t *Libp2pTransport) Start(ctx context.Context) error {
    if !t.cfg.Enable {
        return nil
    }
    opts := []libp2p.Option{}
    if len(t.cfg.Listen) > 0 {
        var addrs []ma.Multiaddr
        for _, s := range t.cfg.Listen {
            if strings.TrimSpace(s) == "" { continue }
            a, err := ma.NewMultiaddr(s); if err != nil { return err }
            addrs = append(addrs, a)
        }
        if len(addrs) > 0 { opts = append(opts, libp2p.ListenAddrs(addrs...)) }
    }
    if t.cfg.NAT { opts = append(opts, libp2p.NATPortMap()) }
    h, err := libp2p.New(opts...)
    if err != nil { return err }
    t.host = h
    ps, err := pubsub.NewGossipSub(ctx, h)
    if err != nil { return err }
    t.ps = ps
    if t.tq, err = ps.Join(wire.TopicQBFT); err != nil { return err }
    if t.tt, err = ps.Join(wire.TopicTx); err != nil { return err }
    if t.subQ, err = t.tq.Subscribe(); err != nil { return err }
    if t.subTx, err = t.tt.Subscribe(); err != nil { return err }

    // connect bootnodes (best effort)
    for _, b := range t.cfg.Bootnodes {
        if strings.TrimSpace(b) == "" { continue }
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
    logger.InfoJ("p2p_start", map[string]any{"result":"ok"})
    return nil
}

func (t *Libp2pTransport) Stop(ctx context.Context) error {
    if t.subQ != nil { _ = t.subQ.Cancel() }
    if t.subTx != nil { _ = t.subTx.Cancel() }
    if t.tq != nil { _ = t.tq.Close() }
    if t.tt != nil { _ = t.tt.Close() }
    if t.host != nil { return t.host.Close() }
    return nil
}

func (t *Libp2pTransport) BroadcastQBFT(_ context.Context, msg qbft.Message) error {
    if t.tq == nil { return errors.New("p2p not started") }
    w := wire.FromInternal(msg)
    b, err := json.Marshal(w)
    if err != nil { return err }
    if err := t.tq.Publish(context.Background(), b); err != nil {
        metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction":"tx", "result":"error"})
        return err
    }
    metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction":"tx", "result":"ok"})
    metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicQBFT, "direction":"tx"})
    return nil
}

func (t *Libp2pTransport) BroadcastTx(_ context.Context, pl payload.Payload) error {
    if t.tt == nil { return errors.New("p2p not started") }
    w, ok := wire.TxFromInternal(pl)
    if !ok { return nil }
    b, err := json.Marshal(w)
    if err != nil { return err }
    if err := t.tt.Publish(context.Background(), b); err != nil {
        metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTx, "direction":"tx", "result":"error"})
        return err
    }
    metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTx, "direction":"tx", "result":"ok"})
    metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicTx, "direction":"tx"})
    return nil
}

func (t *Libp2pTransport) OnQBFT(fn func(qbft.Message))         { t.onQBFT = fn }
func (t *Libp2pTransport) OnTx(fn func(payload.Payload))         { t.onTx = fn }

func (t *Libp2pTransport) loopQBFT(ctx context.Context) {
    for {
        m, err := t.subQ.Next(ctx)
        if err != nil { return }
        b := m.Data
        var w wire.QBFT
        if err := json.Unmarshal(b, &w); err != nil {
            metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction":"rx", "result":"decode_error"})
            continue
        }
        metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicQBFT, "direction":"rx", "result":"ok"})
        metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicQBFT, "direction":"rx"})
        if t.onQBFT != nil { t.onQBFT(w.ToInternal()) }
    }
}

func (t *Libp2pTransport) loopTx(ctx context.Context) {
    for {
        m, err := t.subTx.Next(ctx)
        if err != nil { return }
        b := m.Data
        var w wire.PlaintextTx
        if err := json.Unmarshal(b, &w); err != nil {
            metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTx, "direction":"rx", "result":"decode_error"})
            continue
        }
        metrics.Inc(MetricP2PMessagesTotal, map[string]string{"topic": wire.TopicTx, "direction":"rx", "result":"ok"})
        metrics.Inc(MetricP2PBytesTotal, map[string]string{"topic": wire.TopicTx, "direction":"rx"})
        if t.onTx != nil { if pl := w.ToInternal(); pl != nil { t.onTx(pl) } }
    }
}

func connectOnce(ctx context.Context, h p2phost.Host, addr string) error {
    maAddr, err := ma.NewMultiaddr(addr); if err != nil { return err }
    info, err := peer.AddrInfoFromP2pAddr(maAddr); if err != nil { return err }
    ctx2, cancel := context.WithTimeout(ctx, 5*time.Second); defer cancel()
    return h.Connect(ctx2, *info)
}

