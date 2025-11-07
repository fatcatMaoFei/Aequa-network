package consensus

import (
    "testing"
    "os"
)

type stubTSS struct{ ok bool }
func (s stubTSS) VerifyAgg(_ []byte, _ []byte, _ []byte) bool { return s.ok }

func TestService_TSSStateSync_DisabledByDefault(t *testing.T) {
    svc := New()
    if svc.VerifyHeaderWithTSS(nil, nil, nil) {
        t.Fatalf("should be disabled by default")
    }
}

func TestService_TSSStateSync_EnabledAndVerified(t *testing.T) {
    os.Setenv("AEQUA_ENABLE_TSS_STATE_SYNC", "1")
    t.Cleanup(func(){ os.Unsetenv("AEQUA_ENABLE_TSS_STATE_SYNC") })
    svc := New()
    svc.SetTSSVerifier(stubTSS{ok:true})
    // emulate Start to read env flag
    svc.enableTSSSync = true
    if !svc.VerifyHeaderWithTSS([]byte("pk"), []byte("hdr"), []byte("sig")) {
        t.Fatalf("expected verified ok")
    }
}

