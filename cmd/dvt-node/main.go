package main

import (
    "context"
    "flag"
    "os"
    "os/signal"
    "syscall"

    "github.com/zmlAEQ/Aequa-network/internal/api"
    "github.com/zmlAEQ/Aequa-network/internal/consensus"
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
    )
    flag.StringVar(&apiAddr, "validator-api", "127.0.0.1:4600", "Validator API listen address")
    flag.StringVar(&monAddr, "monitoring", "127.0.0.1:4620", "Monitoring listen address")
    flag.StringVar(&upstream, "upstream", "", "Optional upstream base URL for proxying non-critical requests")
    flag.BoolVar(&enableTSS, "enable-tss", false, "Enable TSS session service (behind feature flag)")
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

    if err := m.StartAll(ctx); err != nil {
        logger.Error(err.Error())
        os.Exit(1)
    }
    <-ctx.Done()
    _ = m.StopAll(context.Background())
}