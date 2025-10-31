package dkg

// 本文件定义 DKG 协议的最小接口与类型占位，用于后续实现 Feldman/Pedersen 流程。

// MessageType 表示 DKG 轮次消息类型。
type MessageType string

const (
    MsgPropose   MessageType = "propose"
    MsgCommit    MessageType = "commit"
    MsgReveal    MessageType = "reveal"
    MsgAck       MessageType = "ack"
    MsgComplaint MessageType = "complaint"
)

// Message 是 DKG 网络消息的最小占位结构。
type Message struct {
    Type      MessageType `json:"type"`
    SessionID string      `json:"session_id"`
    Epoch     uint64      `json:"epoch"`
    From      string      `json:"from"`
    Payload   []byte      `json:"payload"`
    Sig       []byte      `json:"sig,omitempty"`
    TraceID   string      `json:"trace_id,omitempty"`
}

// KeyShare 表示单个参与者的阈值密钥材料（占位）。
type KeyShare struct {
    Index       int      `json:"index"`
    PublicKey   []byte   `json:"public_key"`
    PrivateKey  []byte   `json:"private_key"`
    Commitments [][]byte `json:"commitments"`
}

// Engine 定义最小化 DKG 引擎接口（占位）。
type Engine interface {
    // OnMessage 处理 DKG 消息，返回是否状态前进。
    OnMessage(msg Message) (advanced bool, err error)
}

