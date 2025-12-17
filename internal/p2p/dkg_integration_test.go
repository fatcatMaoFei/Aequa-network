package p2p

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zmlAEQ/Aequa-network/pkg/metrics"
)

type denySome struct{}

func (denySome) VerifyCluster() error     { return nil }
func (denySome) AllowPeer(id string) bool { return id != "B" }

type errCluster struct{}

func (errCluster) VerifyCluster() error     { return errors.New("bad cluster") }
func (errCluster) AllowPeer(id string) bool { return true }

func TestConnect_DKGDeniedLabels(t *testing.T) {
	metrics.Reset()
	s := New()
	s.SetDKG(denySome{})
	if err := s.Connect("A"); err != nil {
		t.Fatalf("A should pass: %v", err)
	}
	if err := s.Connect("B"); err == nil {
		t.Fatalf("B should be dkg denied")
	}
	dump := metrics.DumpProm()
	if !strings.Contains(dump, `p2p_conn_attempts_total{result="allowed"} 1`) {
		t.Fatalf("want allowed=1, got %q", dump)
	}
	if !strings.Contains(dump, `p2p_conn_attempts_total{result="dkg_denied"} 1`) {
		t.Fatalf("want dkg_denied=1, got %q", dump)
	}
}

func TestStart_ClusterVerifyMetrics(t *testing.T) {
	metrics.Reset()
	s := New()
	s.SetDKG(errCluster{})
	// Start triggers cluster verification once.
	if err := s.Start(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
	dump := metrics.DumpProm()
	if !strings.Contains(dump, `p2p_dkg_cluster_checks_total{result="error"} 1`) {
		t.Fatalf("want cluster check error count=1, got %q", dump)
	}
}
