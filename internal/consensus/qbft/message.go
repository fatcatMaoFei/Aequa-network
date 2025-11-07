package qbft

type Type string

const (
    MsgPreprepare Type = "preprepare"
    MsgPrepare   Type = "prepare"
    MsgCommit    Type = "commit"
    MsgViewChange Type = "view_change"
    MsgNewView    Type = "new_view"
)

type Message struct {
    From    string
    Height  uint64
    Round   uint64
    Type    Type
    Payload []byte
    ID      string
    TraceID string
    // Sig is a placeholder for a signature/aggregate signature byte slice.
    // For now, the verifier仅检查形状（长度阈值），不做密码学验真。
    Sig     []byte
}
