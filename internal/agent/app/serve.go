package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/local"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/backup"
	"github.com/QwikByte/noryx/internal/agent/files"
	agentlogs "github.com/QwikByte/noryx/internal/agent/logs"
	"github.com/QwikByte/noryx/internal/agent/network"
	"github.com/QwikByte/noryx/internal/agent/node"
	"github.com/QwikByte/noryx/internal/agent/plugin"
	"github.com/QwikByte/noryx/internal/agent/properties"
	"github.com/QwikByte/noryx/internal/agent/runtime"
	"github.com/QwikByte/noryx/internal/agent/runtime/docker"
	"github.com/QwikByte/noryx/internal/agent/server"
	"github.com/QwikByte/noryx/internal/agent/stats"
	"github.com/QwikByte/noryx/internal/agent/storage"
	"github.com/QwikByte/noryx/internal/buildinfo"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/pki"
)

func serve(ctx context.Context, cfg config) error {
	buf := agentlogs.NewBuffer()
	logFile, err := logging.Setup(cfg.log, buf.Handler(cfg.log.Level))
	if err != nil {
		return err
	}
	defer logFile.Close()
	if err := ensureDataDir(cfg); err != nil {
		return err
	}
	identity, err := node.LoadIdentity(cfg.pkiDir())
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("this agent is not enrolled yet, run: noryx-agent enroll <join-token>")
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
	svc := newServices(rt, identity, locations, buf, slog.Default())
	remote := svc.grpcServer(agentlogs.FromMaster, grpc.Creds(credentials.NewTLS(pki.AgentServerTLS(identity.Holder, identity.CA))))
	localSrv := svc.grpcServer(agentlogs.FromLocal, grpc.Creds(local.NewCredentials()))
	go svc.stats.Run(ctx)
	go rt.Watch(ctx)

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
	slog.Info("Agent started", "version", buildinfo.Version, "listen", cfg.listenAddr, "socket", cfg.socket(),
		"identity", identity.Get().Leaf.Subject.CommonName, "certificate_not_after", identity.Get().Leaf.NotAfter,
		"ca_fingerprint", pki.Fingerprint(identity.CA))

	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	// Running servers are not affected: containers keep running without the agent.
	remote.Stop()
	localSrv.Stop()
	if err != nil {
		slog.Error("Agent stopped", "err", err)
	} else {
		slog.Info("Agent stopped")
	}
	return err
}

// services are the gRPC services of the agent. The servers for the master and the local CLI
// share them, so both see the same state, e.g. a backup in progress.
type services struct {
	rt         runtime.Runtime
	node       *node.Service
	server     *server.Service
	files      *files.Service
	properties *properties.Service
	proxy      *network.Service
	plugin     *plugin.Service
	backup     *backup.Service
	stats      *stats.Service
	log        *agentlogs.Service
	calls      *slog.Logger
}

// newServices returns the services, which log the calls they receive to calls.
func newServices(rt runtime.Runtime, identity *node.Identity, locations *storage.Locations, buf *agentlogs.Buffer, calls *slog.Logger) *services {
	backups := backup.NewService(rt, locations)
	return &services{
		rt:         rt,
		node:       node.NewService(rt, identity, locations),
		server:     server.NewService(rt, backups),
		files:      files.NewService(rt),
		properties: properties.NewService(rt),
		proxy:      network.NewService(rt),
		plugin:     plugin.NewService(rt),
		backup:     backups,
		stats:      stats.NewService(rt),
		log:        agentlogs.NewService(buf),
		calls:      calls,
	}
}

// grpcServer serves the services to callers from origin, logging their calls. If a call failed
// because the runtime is down, the log keeps the original error and the caller learns that.
func (s *services) grpcServer(origin string, opts ...grpc.ServerOption) *grpc.Server {
	opts = slices.Concat(runtime.Interceptors(s.rt), agentlogs.Interceptors(origin, s.calls), opts)
	srv := grpc.NewServer(opts...)
	noryxv1.RegisterNodeServiceServer(srv, s.node)
	noryxv1.RegisterServerServiceServer(srv, s.server)
	noryxv1.RegisterFileServiceServer(srv, s.files)
	noryxv1.RegisterPropertiesServiceServer(srv, s.properties)
	noryxv1.RegisterProxyServiceServer(srv, s.proxy)
	noryxv1.RegisterPluginServiceServer(srv, s.plugin)
	noryxv1.RegisterBackupServiceServer(srv, s.backup)
	noryxv1.RegisterStatsServiceServer(srv, s.stats)
	noryxv1.RegisterLogServiceServer(srv, s.log)
	return srv
}

// NewGRPCServer registers all agent services on a new gRPC server for the master. The calls
// are logged to buf, which the log service reads.
func NewGRPCServer(rt runtime.Runtime, identity *node.Identity, locations *storage.Locations, buf *agentlogs.Buffer, opts ...grpc.ServerOption) *grpc.Server {
	calls := slog.New(buf.Handler(slog.LevelDebug))
	return newServices(rt, identity, locations, buf, calls).grpcServer(agentlogs.FromMaster, opts...)
}
