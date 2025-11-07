package p2p

import (
    "context"

    "github.com/zmlAEQ/Aequa-network/pkg/lifecycle"
)

// NetService is a thin lifecycle wrapper for a Transport.
type NetService struct{ t Transport }

func NewNetService(t Transport) *NetService { return &NetService{t: t} }
func (s *NetService) Name() string          { return "p2p-transport" }
func (s *NetService) Start(ctx context.Context) error { return s.t.Start(ctx) }
func (s *NetService) Stop(ctx context.Context) error  { return s.t.Stop(ctx) }

var _ lifecycle.Service = (*NetService)(nil)

