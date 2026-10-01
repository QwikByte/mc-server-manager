// Package node implements the NodeService of the agent.
package node

import (
	"context"
	"log/slog"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
)

type Service struct {
	mcsmv1.UnimplementedNodeServiceServer
	rt       runtime.Runtime
	identity *Identity
	storage  *storage.Locations
}

func NewService(rt runtime.Runtime, identity *Identity, locations *storage.Locations) *Service {
	return &Service{rt: rt, identity: identity, storage: locations}
}

// GetInfo succeeds even if the runtime is down, so the panel can tell an
// unreachable agent apart from a broken container runtime.
func (s *Service) GetInfo(ctx context.Context, _ *mcsmv1.GetInfoRequest) (*mcsmv1.GetInfoResponse, error) {
	hostname, _ := os.Hostname()
	res := &mcsmv1.GetInfoResponse{AgentVersion: buildinfo.Version, Hostname: hostname, Runtime: "unavailable"}
	if info, err := s.rt.Info(ctx); err == nil {
		res.Os, res.CpuCount, res.MemoryBytes, res.Runtime = info.OS, info.CPUs, info.MemoryBytes, info.Name
	}
	locations, err := s.storage.List()
	if err != nil {
		slog.Warn("can't read the storage locations", "err", err)
	}
	for _, l := range locations {
		res.Storage = append(res.Storage, &mcsmv1.StorageLocation{Name: l.Name, Path: l.Path, FreeBytes: l.FreeBytes, TotalBytes: l.TotalBytes})
	}
	return res, nil
}

func (s *Service) CreateCSR(context.Context, *mcsmv1.CreateCSRRequest) (*mcsmv1.CreateCSRResponse, error) {
	csr, err := s.identity.createCSR()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &mcsmv1.CreateCSRResponse{CsrDer: csr}, nil
}

func (s *Service) InstallCertificate(_ context.Context, req *mcsmv1.InstallCertificateRequest) (*mcsmv1.InstallCertificateResponse, error) {
	if err := s.identity.install(req.GetCertificateDer()); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "certificate rejected: %v", err)
	}
	slog.Info("installed renewed node certificate", "not_after", s.identity.Get().Leaf.NotAfter)
	return &mcsmv1.InstallCertificateResponse{}, nil
}
