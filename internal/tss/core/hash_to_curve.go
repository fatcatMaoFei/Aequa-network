package core

// HashToCurve 为占位实现：仅校验 dst 的基本合法性并返回未实现错误。
// 后续将替换为常数时间的哈希上曲线实现（见 docs/PLAN_v2.md）。
func HashToCurve(msg []byte, dst string) (Point, error) {
    if !IsValidDST(dst) { return nil, ErrInvalidDST }
    return nil, ErrNotImplemented
}

// ErrInvalidDST 表示域分离标识不合法。
var ErrInvalidDST = ErrInvalidDSTType{}

// ErrInvalidDSTType 提供可识别的错误类型，便于测试与上层分类。
type ErrInvalidDSTType struct{}

func (ErrInvalidDSTType) Error() string { return "invalid dst" }

