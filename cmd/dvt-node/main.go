package main

import (
    "context"
    "flag"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "github.com/zmlAEQ/Aequa-network/internal/api"
    "github.com/zmlAEQ/Aequa-network/internal/consensus"
    qbft "github.com/zmlAEQ/Aequa-network/internal/consensus/qbft"
    "github.com/zmlAEQ/Aequa-network/internal/monitoring"
    "github.com/zmlAEQ/Aequa-network/internal/p2p"
    "github.com/zmlAEQ/Aequa-network/internal/tss"
    "github.com/zmlAEQ/Aequa-network/pkg/bus"
    "github.com/zmlAEQ/Aequa-network/pkg/lifecycle"
    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/trace"
)

func main() {
    var (
        apiAddr   string
        monAddr   string
        upstream  string
        enableTSS bool
        p2pEnable bool
        p2pListen string
        p2pBoot   string
        p2pNAT    bool
    )
    flag.StringVar(&apiAddr, "validator-api", "127.0.0.1:4600", "Validator API listen address")
    flag.StringVar(&monAddr, "monitoring", "127.0.0.1:4620", "Monitoring listen address")
    flag.StringVar(&upstream, "upstream", "", "Optional upstream base URL for proxying non-critical requests")
    flag.BoolVar(&enableTSS, "enable-tss", false, "Enable TSS session service (behind feature flag)")
    flag.BoolVar(&p2pEnable, "p2p.enable", false, "Enable P2P transport (libp2p+gossipsub, behind 'p2p' build tag)")
    flag.StringVar(&p2pListen, "p2p.listen", "", "P2P listen multiaddr (e.g. /ip4/0.0.0.0/tcp/31000)")
    flag.StringVar(&p2pBoot, "p2p.bootnodes", "", "Comma-separated bootnode multiaddrs or path to file")
    flag.BoolVar(&p2pNAT, "p2p.nat", false, "Enable NAT port mapping")
    flag.Parse()

    ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer cancel()

    b := bus.New(256)
    publish := func(ctx context.Context, payload []byte) error {
        tid, _ := trace.FromContext(ctx)
        b.Publish(ctx, bus.Event{Kind: bus.KindDuty, Body: payload, TraceID: tid})
        return nil
    }

    m := lifecycle.New()
    m.Add(api.New(apiAddr, publish, upstream))
    m.Add(monitoring.New(monAddr))
    p2ps := p2p.New()
    m.Add(p2ps)
    if enableTSS {
        m.Add(tss.New(p2ps))
    }
    m.Add(consensus.NewWithSub(b.Subscribe()))

    // Start P2P transport (behind build tag); safe no-op without 'p2p' tag or when disabled.
    if p2pEnable {
        cfg := p2p.NetConfig{Enable: true, NAT: p2pNAT}
        if p2pListen != "" { cfg.Listen = []string{p2pListen} }
        // parse bootnodes: comma list or file path
        if p2pBoot != "" {
            if fi, err := os.Stat(p2pBoot); err == nil && !fi.IsDir() {
                if b, err := os.ReadFile(p2pBoot); err == nil {
                    lines := strings.Split(string(b), "\n")
                    for _, ln := range lines { ln = strings.TrimSpace(ln); if ln != "" { cfg.Bootnodes = append(cfg.Bootnodes, ln) } }
                }
            } else {
                parts := strings.Split(p2pBoot, ",")
                for _, p := range parts { p = strings.TrimSpace(p); if p != "" { cfg.Bootnodes = append(cfg.Bootnodes, p) } }
            }
        }
        if t, _ := p2p.StartTransportIfEnabled(ctx, cfg); t != nil {
            t.OnQBFT(func(m qbft.Message) {
                b.Publish(ctx, bus.Event{Kind: bus.KindConsensus, Height: m.Height, Round: m.Round, Body: m, TraceID: m.TraceID})
            })
            // ensure graceful stop with lifecycle: wrap and add
            m.Add(p2p.NewNetService(t))
        }
    }

    if err := m.StartAll(ctx); err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
    <-ctx.Done()
    _ = m.StopAll(context.Background())
}
