package main

import (
    "bytes"
    "crypto/rand"
    "encoding/binary"
    "encoding/json"
    "fmt"
    mrand "math/rand"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"
)

// qbft message used by /e2e/qbft endpoint
type qbftMsg struct {
    From    string `json:"From"`
    Height  uint64 `json:"Height"`
    Round   uint64 `json:"Round"`
    Type    string `json:"Type"`
    Payload []byte `json:"Payload"`
    ID      string `json:"ID"`
    TraceID string `json:"TraceID"`
    Sig     []byte `json:"Sig"`
}

// p2p request used by /e2e/p2p/connect|disconnect endpoints
type p2pReq struct { ID string `json:"id"` }

// txEnvelope is a minimal copy of the public TxEnvelope used by the tx API.
type txEnvelope struct {
    Type         string `json:"type,omitempty"`
    From         string `json:"from"`
    Nonce        uint64 `json:"nonce"`
    Gas          uint64 `json:"gas,omitempty"`
    Fee          uint64 `json:"fee,omitempty"`
    Bid          uint64 `json:"bid,omitempty"`
    FeeRecipient string `json:"fee_recipient,omitempty"`
    Ciphertext   []byte `json:"ciphertext,omitempty"`
    EphemeralKey []byte `json:"ephemeral_key,omitempty"`
    TargetHeight uint64 `json:"target_height,omitempty"`
}

func postJSON(url string, v any) error {
    b, _ := json.Marshal(v)
    resp, err := http.Post(url, "application/json", bytes.NewReader(b))
    if err != nil { return err }
    _ = resp.Body.Close()
    return nil
}

func splitNonEmpty(s string) []string {
    parts := strings.Split(s, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts { p = strings.TrimSpace(p); if p != "" { out = append(out, p) } }
    return out
}

func main() {
    // local RNG seeded from crypto/rand to avoid deprecated global Seed
    var seed int64
    var b8 [8]byte
    if _, err := rand.Read(b8[:]); err == nil { seed = int64(binary.LittleEndian.Uint64(b8[:])) } else { seed = time.Now().UnixNano() }
    r := mrand.New(mrand.NewSource(seed))

    // endpoints
    epsQBFT := os.Getenv("ENDPOINTS_QBFT")
    if epsQBFT == "" { epsQBFT = "http://aequa-node-0:4610,http://aequa-node-1:4610,http://aequa-node-2:4610,http://aequa-node-3:4610" }
    qbftTargets := splitNonEmpty(epsQBFT)
    for i := range qbftTargets { qbftTargets[i] = strings.TrimRight(qbftTargets[i], "/") + "/e2e/qbft" }

    epsP2P := os.Getenv("ENDPOINTS_P2P")
    if epsP2P == "" { epsP2P = "http://aequa-node-0:4615,http://aequa-node-1:4615,http://aequa-node-2:4615,http://aequa-node-3:4615" }
    p2pTargets := splitNonEmpty(epsP2P)
    for i := range p2pTargets { p2pTargets[i] = strings.TrimRight(p2pTargets[i], "/") }

    // p2p storm parameters
    idSpace := os.Getenv("P2P_ID_PREFIX")
    if idSpace == "" { idSpace = "X" }
    maxIDs := 1024
    if v := os.Getenv("P2P_MAX_IDS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 { maxIDs = n }
    }

    // qbft adversarial injection
    go func() {
        var stored *qbftMsg
        ticker := time.NewTicker(1500 * time.Millisecond); defer ticker.Stop()
        for range ticker.C {
            u := qbftTargets[r.Intn(len(qbftTargets))]
            switch r.Intn(6) {
            case 0:
                h := uint64(r.Intn(1000) + 1)
                pre := qbftMsg{From:"L", Type:"preprepare", Height:h, Round:0, ID:fmt.Sprintf("blk-%d", h)}
                _ = postJSON(u, pre)
                cm := qbftMsg{From:"C1", Type:"commit", Height:h, Round:1, ID:pre.ID}
                _ = postJSON(u, cm); stored = &cm
            case 1:
                if stored != nil { _ = postJSON(u, *stored) }
            case 2:
                h := uint64(r.Intn(1000) + 1)
                pre := qbftMsg{From:"L", Type:"preprepare", Height:h, Round:0, ID:fmt.Sprintf("blk-%d", h)}
                _ = postJSON(u, pre)
                _ = postJSON(u, qbftMsg{From:"C2", Type:"commit", Height:h, Round:1, ID:"other"})
            case 3:
                _ = postJSON(u, qbftMsg{From:"P1", Type:"prepare", Height:10, Round:0, ID:"x"})
                _ = postJSON(u, qbftMsg{From:"C1", Type:"commit", Height:11, Round:0, ID:"y"})
            case 4:
                _ = postJSON(u, qbftMsg{From:"S", Type:"prepare", Height:12, Round:1, ID:"z", Sig:[]byte{1,2,3}})
            default:
                h := uint64(r.Intn(1000) + 1)
                pre := qbftMsg{From:"L", Type:"preprepare", Height:h, Round:0, ID:fmt.Sprintf("blk-%d", h)}
                _ = postJSON(u, pre)
                _ = postJSON(u, qbftMsg{From:"P1", Type:"prepare", Height:h, Round:1, ID:pre.ID})
                _ = postJSON(u, qbftMsg{From:"P2", Type:"prepare", Height:h, Round:1, ID:pre.ID})
            }
        }
    }()

    // p2p connect/disconnect storm
    go func() {
        ticker := time.NewTicker(500 * time.Millisecond); defer ticker.Stop()
        counter := 0
        for range ticker.C {
            base := p2pTargets[r.Intn(len(p2pTargets))]
            id := fmt.Sprintf("%s-%d", idSpace, r.Intn(maxIDs))
            if r.Intn(5) > 0 { _ = postJSON(base+"/e2e/p2p/connect", p2pReq{ID:id}) } else { _ = postJSON(base+"/e2e/p2p/disconnect", p2pReq{ID:id}) }
            counter++; if counter%200 == 0 { time.Sleep(2 * time.Second) }
        }
    }()

    // optional BEAST/private_tx adversarial traffic against /v1/tx/plain
    // controlled via ATTACK_BEAST=1 to keep default behaviour unchanged.
    if os.Getenv("ATTACK_BEAST") == "1" {
        epsTX := os.Getenv("ENDPOINTS_TX")
        if epsTX == "" { epsTX = "http://aequa-node-0:4600" }
        txTargets := splitNonEmpty(epsTX)
        for i := range txTargets { txTargets[i] = strings.TrimRight(txTargets[i], "/") + "/v1/tx/plain" }

        go func() {
            ticker := time.NewTicker(2 * time.Second); defer ticker.Stop()
            nonce := uint64(0)
            for range ticker.C {
                u := txTargets[r.Intn(len(txTargets))]
                // random private_v1 ciphertext (may or may not be valid JSON / BEAST payload)
                ciph := make([]byte, 48)
                _, _ = rand.Read(ciph)
                eph := make([]byte, 32)
                _, _ = rand.Read(eph)
                env := txEnvelope{
                    Type:         "private_v1",
                    From:         "attacker",
                    Nonce:        nonce,
                    Ciphertext:   ciph,
                    EphemeralKey: eph,
                    TargetHeight: 100,
                }
                _ = postJSON(u, env)
                nonce++
            }
        }()
    }

    select {}
}
