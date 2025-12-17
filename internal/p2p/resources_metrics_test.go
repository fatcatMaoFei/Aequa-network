package p2p

import (
	"strings"
	"testing"

	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

func TestResourceManager_OpenCloseCounters(t *testing.T) {
	metrics.Reset()
	r := NewResourceManager(ResourceLimits{MaxConns: 2})
	if !r.TryOpen() {
		t.Fatalf("expected first open to succeed")
	}
	if !r.TryOpen() {
		t.Fatalf("expected second open to succeed")
	}
	if r.TryOpen() {
		t.Fatalf("third open should fail due to limit")
	}
	r.Close()
	r.Close()
	dump := metrics.DumpProm()
	if !strings.Contains(dump, "p2p_conn_open_total 2") {
		t.Fatalf("want open_total=2, got %q", dump)
	}
	if !strings.Contains(dump, "p2p_conn_close_total 2") {
		t.Fatalf("want close_total=2, got %q", dump)
	}
}
