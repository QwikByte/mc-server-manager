package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/master/auth"
	"github.com/QwikByte/mc-server-manager/internal/master/backup"
	"github.com/QwikByte/mc-server-manager/internal/master/files"
	"github.com/QwikByte/mc-server-manager/internal/master/modrinth"
	"github.com/QwikByte/mc-server-manager/internal/master/network"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
	"github.com/QwikByte/mc-server-manager/internal/master/plugin"
	"github.com/QwikByte/mc-server-manager/internal/master/policy"
	"github.com/QwikByte/mc-server-manager/internal/master/properties"
	"github.com/QwikByte/mc-server-manager/internal/master/schedule"
	"github.com/QwikByte/mc-server-manager/internal/master/server"
	"github.com/QwikByte/mc-server-manager/internal/master/settings"
	"github.com/QwikByte/mc-server-manager/internal/master/template"
	"github.com/QwikByte/mc-server-manager/internal/master/terminal"
	"github.com/QwikByte/mc-server-manager/internal/pki"
	"github.com/QwikByte/mc-server-manager/web"
)

func serve(ctx context.Context, cfg config) error {
	db, err := openDB(cfg.dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	ca, err := pki.LoadOrCreateCA(filepath.Join(cfg.dataDir, "pki"))
	if err != nil {
		return err
	}
	cert, err := ca.MasterCertificate()
	if err != nil {
		return err
	}
	masterCert, err := pki.NewHolder(cert)
	if err != nil {
		return err
	}
	publicAddr, err := cfg.publicEnrollAddr()
	if err != nil {
		return err
	}

	conf, err := settings.Load(ctx, db, settings.Master{
		Version: buildinfo.Version, StartedAt: time.Now(), PanelAddr: cfg.httpAddr, PanelTLS: cfg.tlsCert != "",
		EnrollListenAddr: cfg.enrollAddr, EnrollAddr: publicAddr, CAFingerprint: pki.Fingerprint(ca.Cert),
	}, masterCert)
	if err != nil {
		return err
	}

	users := auth.NewService(db)
	nodes := node.NewService(db, ca, masterCert, conf)
	defer nodes.Close()
	go masterCert.Maintain(ctx, time.Hour, ca.MasterCertificate)
	go nodes.MaintainCertificates(ctx, 6*time.Hour)
	if ok, err := users.HasUsers(ctx); err == nil && !ok {
		slog.Warn("no administrator account exists yet, create one with: mcsm-master user add <username>")
	}

	enrollListener, err := net.Listen("tcp", cfg.enrollAddr)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(pki.MasterServerTLS(masterCert))))
	mcsmv1.RegisterEnrollmentServiceServer(grpcServer, nodes)
	plugins := plugin.NewService(nodes, modrinth.New(modrinth.DefaultAPI, modrinth.DefaultCDN))
	tasks := schedule.NewService(db, nodes, map[string]schedule.Kind{backup.TaskKind: backup.NewJobs(nodes), policy.TaskKind: policy.New(nodes)})
	if err := tasks.Start(ctx); err != nil {
		return err
	}
	httpServer := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           routes(users, conf, nodes, network.NewService(db, nodes), plugins, template.NewService(db, plugins), tasks),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 2)
	go func() { errc <- grpcServer.Serve(enrollListener) }()
	go func() {
		if cfg.tlsCert != "" {
			errc <- httpServer.ListenAndServeTLS(cfg.tlsCert, cfg.tlsKey)
		} else {
			errc <- httpServer.ListenAndServe()
		}
	}()
	slog.Info("master started", "version", buildinfo.Version, "panel", cfg.httpAddr,
		"enrollment", cfg.enrollAddr, "public_enrollment", conf.EnrollAddr(), "ca_fingerprint", pki.Fingerprint(ca.Cert))

	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	return errors.Join(err, httpServer.Shutdown(shutdownCtx))
}

func routes(users *auth.Service, conf *settings.Service, nodes *node.Service, networks *network.Service,
	plugins *plugin.Service, templates *template.Service, tasks *schedule.Service,
) http.Handler {
	authHandler := auth.NewHandler(users, conf.SessionTTL)
	api := http.NewServeMux()
	authHandler.Register(api)
	settings.NewHandler(conf).Register(api)
	terminal.NewHandler(nodes, conf).Register(api)
	node.NewHandler(nodes).Register(api)
	server.NewHandler(nodes, networks, tasks).Register(api)
	network.NewHandler(networks).Register(api)
	files.NewHandler(nodes).Register(api)
	properties.NewHandler(nodes).Register(api)
	plugin.NewHandler(plugins).Register(api)
	template.NewHandler(templates).Register(api)
	backup.NewHandler(nodes).Register(api)
	schedule.NewHandler(tasks, backup.TaskKind).Register(api, "/api/backup-jobs")
	schedule.NewHandler(tasks, policy.TaskKind).Register(api, "/api/policies")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.Handle("/api/", authHandler.Require(api))
	mux.Handle("/", web.Handler())
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// Scripts are restricted to the bundle; the UI libraries inject <style> elements at runtime.
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (c config) publicEnrollAddr() (string, error) {
	if c.publicAddr != "" {
		return c.publicAddr, nil
	}
	_, port, err := net.SplitHostPort(c.enrollAddr)
	if err != nil {
		return "", err
	}
	host, err := os.Hostname()
	return net.JoinHostPort(host, port), err
}
