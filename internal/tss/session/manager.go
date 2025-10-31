package session

import (
    "context"
    "sync"
    "time"

    "github.com/zmlAEQ/Aequa-network/pkg/logger"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// Phase 表示最小会话阶段。
type Phase string

const (
    PhaseInit    Phase = "init"
    PhaseGather  Phase = "gather"
    PhaseCombine Phase = "combine"
    PhaseDone    Phase = "done"
)

// Config 为会话配置占位。
type Config struct {
    Threshold      int           // 份额阈值（最小 2）
    GatherTimeout  time.Duration // 从 Init 进入 Gather 后的超时
    CombineTimeout time.Duration // 进入 Combine 后的超时（占位）
}

func defaultConfig(c Config) Config {
    if c.Threshold < 2 {
        c.Threshold = 2
    }
    if c.GatherTimeout <= 0 {
        c.GatherTimeout = 2 * time.Second
    }
    if c.CombineTimeout <= 0 {
        c.CombineTimeout = 2 * time.Second
    }
    return c
}

// Manager 提供最小的会话状态机（占位实现）。
type Manager struct {
    mu        sync.Mutex
    cfg       Config
    phase     Phase
    startedAt time.Time
    shares    map[string]struct{} // 去重键：from+"|"+nonce
    timedOut  bool

    // 关闭控制
    ctx    context.Context
    cancel context.CancelFunc
}

// NewManager 构造会话管理器（未启动）。
func NewManager(cfg Config) *Manager {
    cfg = defaultConfig(cfg)
    return &Manager{cfg: cfg, phase: PhaseInit, shares: make(map[string]struct{})}
}

// Start 启动会话时钟与超时监视。
func (m *Manager) Start(ctx context.Context) {
    m.mu.Lock()
    if m.ctx != nil { m.mu.Unlock(); return }
    m.ctx, m.cancel = context.WithCancel(ctx)
    m.startedAt = time.Now()
    m.mu.Unlock()

    go m.watchdog()
}

// Stop 结束会话。
func (m *Manager) Stop() {
    m.mu.Lock(); if m.cancel != nil { m.cancel() }; m.mu.Unlock()
}

func (m *Manager) watchdog() {
    // 仅处理 Gather 超时（占位）。
    t := time.NewTicker(10 * time.Millisecond)
    defer t.Stop()
    for {
        select {
        case <-m.ctx.Done():
            return
        case <-t.C:
            m.mu.Lock()
            if m.phase == PhaseGather {
                if time.Since(m.startedAt) >= m.cfg.GatherTimeout {
                    m.timedOut = true
                    // 记录会话结果与阶段延迟
                    metrics.Inc("tss_sessions_total", map[string]string{"result": "timeout"})
                    metrics.ObserveSummary("tss_round_ms", map[string]string{"round": string(PhaseGather)}, float64(time.Since(m.startedAt).Milliseconds()))
                    logger.ErrorJ("tss_session", map[string]any{"event": "timeout", "phase": string(m.phase), "latency_ms": time.Since(m.startedAt).Milliseconds(), "trace_id": ""})
                    m.phase = PhaseDone
                }
            }
            m.mu.Unlock()
        }
    }
}

// OnShare 注入去重后的份额（占位语义：达到阈值进入 Combine）。
func (m *Manager) OnShare(from, nonce string, payload []byte) (bool, error) {
    key := from + "|" + nonce
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.phase == PhaseDone {
        return false, nil
    }
    if m.phase == PhaseInit {
        m.phase = PhaseGather
        m.startedAt = time.Now()
        // 记录进入 Gather 的瞬间（init 轮延迟）
        metrics.ObserveSummary("tss_round_ms", map[string]string{"round": string(PhaseInit)}, 0)
        logger.InfoJ("tss_session", map[string]any{"event": "phase", "phase": string(m.phase), "trace_id": ""})
    }
    if _, ok := m.shares[key]; ok {
        // 去重：不推进
        return false, nil
    }
    m.shares[key] = struct{}{}
    if len(m.shares) >= m.cfg.Threshold && m.phase == PhaseGather {
        // 达到阈值进入 Combine
        m.phase = PhaseCombine
        metrics.ObserveSummary("tss_round_ms", map[string]string{"round": string(PhaseGather)}, float64(time.Since(m.startedAt).Milliseconds()))
        logger.InfoJ("tss_session", map[string]any{"event": "phase", "phase": string(m.phase), "trace_id": ""})
        return true, nil
    }
    return false, nil
}

// Finalize 将会话结束为 Done，并记录结果。
func (m *Manager) Finalize() {
    m.mu.Lock()
    if m.phase == PhaseCombine {
        metrics.ObserveSummary("tss_round_ms", map[string]string{"round": string(PhaseCombine)}, float64(time.Since(m.startedAt).Milliseconds()))
        metrics.Inc("tss_sessions_total", map[string]string{"result": "ok"})
        logger.InfoJ("tss_session", map[string]any{"event": "finish", "result": "ok", "trace_id": ""})
        m.phase = PhaseDone
    }
    m.mu.Unlock()
}

// Status 返回只读状态快照。
type Status struct { Phase Phase; TimedOut bool }

func (m *Manager) Status() Status {
    m.mu.Lock(); defer m.mu.Unlock()
    return Status{Phase: m.phase, TimedOut: m.timedOut}
}

