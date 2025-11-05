package dkg

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestDKG_CompleteRound_N4T3_Persists(t *testing.T) {
    dir := t.TempDir()
    store := NewKeyStore(filepath.Join(dir, "tss_keyshare.dat"))
    eng := NewEngine(Config{N: 4, T: 3, Store: store})
    sid := "s1"

    // Propose from 3 nodes
    _, _ = eng.OnMessage(Message{Type: MsgPropose, SessionID: sid, Epoch: 1, From: "P1"})
    _, _ = eng.OnMessage(Message{Type: MsgPropose, SessionID: sid, Epoch: 1, From: "P2"})
    _, _ = eng.OnMessage(Message{Type: MsgPropose, SessionID: sid, Epoch: 1, From: "P3"})
    // Commit from 3 nodes
    _, _ = eng.OnMessage(Message{Type: MsgCommit, SessionID: sid, Epoch: 1, From: "P1"})
    _, _ = eng.OnMessage(Message{Type: MsgCommit, SessionID: sid, Epoch: 1, From: "P2"})
    _, _ = eng.OnMessage(Message{Type: MsgCommit, SessionID: sid, Epoch: 1, From: "P3"})
    // Reveal from 3 nodes
    _, _ = eng.OnMessage(Message{Type: MsgReveal, SessionID: sid, Epoch: 1, From: "P1"})
    _, _ = eng.OnMessage(Message{Type: MsgReveal, SessionID: sid, Epoch: 1, From: "P2"})
    _, _ = eng.OnMessage(Message{Type: MsgReveal, SessionID: sid, Epoch: 1, From: "P3"})

    // First two ACKs should not finalize
    adv, _ := eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P1"})
    if adv { t.Fatalf("unexpected advance on first ack") }
    adv, _ = eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P2"})
    if adv { t.Fatalf("unexpected advance on second ack") }
    // Third ACK reaches threshold -> finalize and persist
    adv, _ = eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P3"})
    if !adv { t.Fatalf("want advance on threshold ack") }

    // Verify persistence by loading from store
    if _, err := store.LoadKeyShare(context.Background()); err != nil {
        t.Fatalf("load persisted keyshare: %v", err)
    }
}

func TestDKG_Complaint_NoAdvance(t *testing.T) {
    eng := NewEngine(Config{N: 4, T: 3})
    adv, err := eng.OnMessage(Message{Type: MsgComplaint, SessionID: "sC", Epoch: 1, From: "P1"})
    if err != nil { t.Fatalf("err: %v", err) }
    if adv { t.Fatalf("complaint must not advance") }
}

func TestDKG_EpochMonotonicity(t *testing.T) {
    eng := NewEngine(Config{N: 4, T: 2})
    sid := "sE"
    // Epoch 2 arrives first, then epoch 1 is ignored
    _, _ = eng.OnMessage(Message{Type: MsgPropose, SessionID: sid, Epoch: 2, From: "P1"})
    _, _ = eng.OnMessage(Message{Type: MsgPropose, SessionID: sid, Epoch: 1, From: "P2"})
    // Complete with epoch 2 messages
    _, _ = eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 2, From: "P1"})
    adv, _ := eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 2, From: "P2"})
    if !adv { t.Fatalf("want advance with epoch 2 acks") }
}

func TestDKG_PersistOptionally(t *testing.T) {
    // When no store is provided, finalize should not fail
    eng := NewEngine(Config{N: 3, T: 2})
    sid := "sN"
    _, _ = eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P1"})
    adv, _ := eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P2"})
    if !adv { t.Fatalf("want advance without store") }

    // With store provided, ensure file created
    dir := t.TempDir()
    store := NewKeyStore(filepath.Join(dir, "tss_keyshare.dat"))
    eng2 := NewEngine(Config{N: 3, T: 2, Store: store})
    sid2 := "sM"
    _, _ = eng2.OnMessage(Message{Type: MsgAck, SessionID: sid2, Epoch: 1, From: "P1"})
    adv, _ = eng2.OnMessage(Message{Type: MsgAck, SessionID: sid2, Epoch: 1, From: "P2"})
    if !adv { t.Fatalf("want advance with store") }
    if _, err := os.Stat(filepath.Join(dir, "tss_keyshare.dat")); err != nil {
        t.Fatalf("expected persisted file: %v", err)
    }
}


func TestDKG_Complaint_TriggersRetryAndFinalizeAfter(t *testing.T) {
    dir := t.TempDir()
    store := NewKeyStore(filepath.Join(dir, "tss_keyshare.dat"))
    sess := NewSessionStore(dir)
    eng := NewEngine(Config{N:4, T:2, Store: store, Sess: sess})
    sid := "sr"
    // progress then complaint -> epoch++ and reset
    _, _ = eng.OnMessage(Message{Type: MsgPropose, SessionID: sid, Epoch: 1, From: "P1"})
    _, _ = eng.OnMessage(Message{Type: MsgCommit, SessionID: sid, Epoch: 1, From: "P1"})
    _, _ = eng.OnMessage(Message{Type: MsgComplaint, SessionID: sid, Epoch: 1, From: "P2"})
    // new epoch finalize with two ACKs
    _, _ = eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 2, From: "P1"})
    adv, _ := eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 2, From: "P2"})
    if !adv { t.Fatalf("want advance after retry") }
}

func TestDKG_Resume_FromSessionStore(t *testing.T) {
    dir := t.TempDir()
    store := NewKeyStore(filepath.Join(dir, "tss_keyshare.dat"))
    sess := NewSessionStore(dir)
    sid := "sx"
    // first engine writes partial state (one ack)
    eng := NewEngine(Config{N:3, T:2, Store: store, Sess: sess})
    _, _ = eng.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P1"})
    // resume into new engine
    eng2 := NewEngine(Config{N:3, T:2, Store: store, Sess: sess})
    if err := eng2.Resume(sid); err != nil { t.Fatalf("resume: %v", err) }
    adv, _ := eng2.OnMessage(Message{Type: MsgAck, SessionID: sid, Epoch: 1, From: "P2"})
    if !adv { t.Fatalf("want finalize after resume") }
}