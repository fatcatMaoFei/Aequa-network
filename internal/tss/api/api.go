package api

import (
    "context"
    "errors"
)

// 占位错误类型：尚未实现。
var ErrNotImplemented = errors.New("not implemented")

// Duty 描述最小签名职责坐标（与现有共识高度/轮次对齐）。
type Duty struct {
    Height uint64
    Round  uint64
}

// Service 提供最小化 TSS API，占位实现不做任何签名或验证。
type Service struct{}

// New 返回占位 Service。
func New() *Service { return &Service{} }

// Sign 启动（或加入）一次阈值签名会话并返回聚合签名（占位：返回未实现）。
func (s *Service) Sign(ctx context.Context, duty Duty, msg []byte) ([]byte, error) {
    return nil, ErrNotImplemented
}

// VerifyAgg 验证聚合签名（占位：恒 false）。
func (s *Service) VerifyAgg(pkGroup []byte, msg []byte, aggSig []byte) bool {
    return false
}

// Resume 恢复指定会话（占位：返回未实现）。
func (s *Service) Resume(sessionID string) error { return ErrNotImplemented }

