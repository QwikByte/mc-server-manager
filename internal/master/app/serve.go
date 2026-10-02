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
	"github.com/QwikByte/mc-server-manager/internal/logging"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/auth"
	"github.com/QwikByte/mc-server-manager/internal/master/backup"
	"github.com/QwikByte/mc-server-manager/internal/master/files"
	"github.com/QwikByte/mc-server-manager/internal/master/logs"
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
	logStore := logs.NewStore(db, logs.NewNames(nodes), conf.LogRetention)
	logFile, err := logging.Setup(cfg.log, logStore.Handler(cfg.log.Level))
	if err != nil {
		return err
	}
	defer logFile.Close()
	logStore.Start(ctx)
	defer logStore.Close() // before the log file and the database close
	go logStore.Collect(ctx, nodes)
	go masterCert.Maintain(ctx, time.Hour, ca.MasterCertificate)
	go nodes.MaintainCertificates(ctx, 6*time.Hour)
	if ok, err := users.HasUsers(ctx); err == nil && !ok {
		slog.Warn("No administrator account exists yet, create one with: mcsm-master user add <username>", logging.Auth)
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
		Addr: cfg.httpAddr,
		Handler: Handler(Services{
			Users: users, Access: access.NewService(db), Settings: conf, Nodes: nodes, Networks: network.NewService(db, nodes),
			Plugins: plugins, Templates: template.NewService(db, plugins), Tasks: tasks, Logs: logStore,
		}),
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
	slog.Info("Master started", "version", buildinfo.Version, "panel", cfg.httpAddr,
		"enrollment", cfg.enrollAddr, "public_enrollment", conf.EnrollAddr(), "ca_fingerprint", pki.Fingerprint(ca.Cert))

	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	err = errors.Join(err, httpServer.Shutdown(shutdownCtx))
	if err != nil {
		slog.Error("Master stopped", "err", err)
	} else {
		slog.Info("Master stopped")
	}
	return err
}

// Services are what the master serves over HTTP.
type Services struct {
	Users     *auth.Service
	Access    *access.Service
	Settings  *settings.Service
	Nodes     *node.Service
	Networks  *network.Service
	Plugins   *plugin.Service
	Templates *template.Service
	Tasks     *schedule.Service
	Logs      *logs.Store
}

// Handler returns everything the master serves over HTTP: the panel, the sign-in and the
// API, which needs a session and loads the user's permissions for every request.
func Handler(s Services) http.Handler {
	authHandler := auth.NewHandler(s.Users, s.Settings.SessionTTL)
	api := API(s)
	authHandler.Register(api)

	mux := http.NewServeMux()
	authHandler.RegisterPublic(mux)
	mux.Handle("/api/", authHandler.Require(s.Access.Middleware(api)))
	mux.Handle("/", web.Handler())
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux))
}

// API returns the routes of the API that act for a user, each with the permission it needs.
// Requests that change something are logged.
func API(s Services) *http.ServeMux {
	api := http.NewServeMux()
	m := access.NewMux(api, s.Logs.Audit())
	access.NewHandler(s.Access, s.Users).Register(m)
	settings.NewHandler(s.Settings).Register(m)
	logs.NewHandler(s.Logs).Register(m)
	terminal.NewHandler(s.Nodes, s.Settings, s.Logs).Register(m)
	node.NewHandler(s.Nodes).Register(m)
	server.NewHandler(s.Nodes, s.Networks, s.Tasks, s.Access).Register(m)
	network.NewHandler(s.Networks).Register(m)
	files.NewHandler(s.Nodes).Register(m)
	properties.NewHandler(s.Nodes).Register(m)
	plugin.NewHandler(s.Plugins).Register(m)
	template.NewHandler(s.Templates).Register(m)
	backup.NewHandler(s.Nodes).Register(m)
	schedule.NewHandler(s.Tasks, backup.TaskKind, access.BackupJobsView, access.BackupJobsManage).Register(m, "/api/backup-jobs")
	schedule.NewHandler(s.Tasks, policy.TaskKind, access.PoliciesView, access.PoliciesManage).Register(m, "/api/policies")
	return api
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
