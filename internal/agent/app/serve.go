package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/local"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/node"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime/docker"
	"github.com/QwikByte/mc-server-manager/internal/agent/server"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/pki"
)

func serve(ctx context.Context, cfg config) error {
	if err := ensureDataDir(cfg); err != nil {
		return err
	}
	identity, err := node.LoadIdentity(cfg.pkiDir())
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("this agent is not enrolled yet, run: mcsm-agent enroll <join-token>")
	}
	if err != nil {
		return err
	}
	locations := storage.New(cfg.dataDir)
	rt, err := docker.New(locations)
	if err != nil {
		return err
	}
	defer rt.Close()

	// The master connects via mutual TLS, the local CLI via the Unix socket.
	remote := NewGRPCServer(rt, identity, locations, grpc.Creds(credentials.NewTLS(pki.AgentServerTLS(identity.Holder, identity.CA))))
	localSrv := NewGRPCServer(rt, identity, locations, grpc.Creds(local.NewCredentials()))

	tcpListener, err := net.Listen("tcp", cfg.listenAddr)
	if err != nil {
		return err
	}
	if err := os.Remove(cfg.socket()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	unixListener, err := net.Listen("unix", cfg.socket())
	if err != nil {
		return fmt.Errorf("%w (choose a shorter path with --socket)", err)
	}
	if err := os.Chmod(cfg.socket(), 0o600); err != nil {
		return err
	}

	errc := make(chan error, 2)
	go func() { errc <- remote.Serve(tcpListener) }()
	go func() { errc <- localSrv.Serve(unixListener) }()
	slog.Info("agent started", "version", buildinfo.Version, "listen", cfg.listenAddr, "socket", cfg.socket(),
		"node", identity.Get().Leaf.Subject.CommonName, "certificate_not_after", identity.Get().Leaf.NotAfter,
		"ca_fingerprint", pki.Fingerprint(identity.CA))

	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	// Running servers are not affected: containers keep running without the agent.
	remote.Stop()
	localSrv.Stop()
	return err
}

// NewGRPCServer registers all agent services on a new gRPC server.
func NewGRPCServer(rt runtime.Runtime, identity *node.Identity, locations *storage.Locations, opts ...grpc.ServerOption) *grpc.Server {
	s := grpc.NewServer(opts...)
	mcsmv1.RegisterNodeServiceServer(s, node.NewService(rt, identity, locations))
	mcsmv1.RegisterServerServiceServer(s, server.NewService(rt))
	return s
}
