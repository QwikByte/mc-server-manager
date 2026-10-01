// Package node implements the NodeService of the agent.
package node

import (
	"context"
	"os"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
)

type Service struct {
	mcsmv1.UnimplementedNodeServiceServer
	rt runtime.Runtime
}

func NewService(rt runtime.Runtime) *Service { return &Service{rt: rt} }

// GetInfo succeeds even if the runtime is down, so the panel can tell an
// unreachable agent apart from a broken container runtime.
func (s *Service) GetInfo(ctx context.Context, _ *mcsmv1.GetInfoRequest) (*mcsmv1.GetInfoResponse, error) {
	hostname, _ := os.Hostname()
	res := &mcsmv1.GetInfoResponse{AgentVersion: buildinfo.Version, Hostname: hostname, Runtime: "unavailable"}
	if info, err := s.rt.Info(ctx); err == nil {
		res.Os, res.CpuCount, res.MemoryBytes, res.Runtime = info.OS, info.CPUs, info.MemoryBytes, info.Name
	}
	return res, nil
}
