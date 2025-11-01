package p2p

import (
    "sync/atomic"
    "github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

// TSSLimiter 提供会话并发数上限控制（节点级）。
type TSSLimiter struct{
    max  int64
    open int64
}

func NewTSSLimiter(max int64) *TSSLimiter { return &TSSLimiter{max: max} }

// TryOpen 尝试打开一个 TSS 会话；超过上限则记录限流指标并返回 false。
func (l *TSSLimiter) TryOpen() bool {
    if l == nil || l.max <= 0 { return true }
    for {
        o := atomic.LoadInt64(&l.open)
        if o >= l.max {
            metrics.Inc("tss_rate_limited_total", map[string]string{"kind": "tss_session"})
            return false
        }
        if atomic.CompareAndSwapInt64(&l.open, o, o+1) {
            metrics.AddGauge("tss_sessions_open", nil, 1)
            return true
        }
    }
}

// Close 关闭一个会话并更新计数。
func (l *TSSLimiter) Close() {
    if l == nil || l.max <= 0 { return }
    for {
        o := atomic.LoadInt64(&l.open)
        if o <= 0 { return }
        if atomic.CompareAndSwapInt64(&l.open, o, o-1) {
            metrics.AddGauge("tss_sessions_open", nil, -1)
            return
        }
    }
}

