package core

import "errors"

// 占位错误：用于尚未实现的密码学原语。
var ErrNotImplemented = errors.New("not implemented")

// 域分离常量（与 PLAN_v2 对齐）。
const (
    DSTSig = "EQS/TSS/v1/SIG" // 阈值签名域
    DSTDkg = "EQS/TSS/v1/DKG" // DKG 域
    DSTApp = "EQS/APP/v1/MSG" // 应用消息域（被签消息）
)

// Point/Scalar 仅作形状占位，不包含任何曲线实现。
type (
    Scalar []byte
    Point  []byte
)

// Group 定义最小化群接口。MVP 仅暴露 HashToCurve，占位实现返回未实现错误。
type Group interface {
    HashToCurve(msg []byte, dst string) (Point, error)
}

// IsValidDST 检查 dst 是否属于预设域分离常量。
func IsValidDST(dst string) bool { return dst == DSTSig || dst == DSTDkg || dst == DSTApp }

