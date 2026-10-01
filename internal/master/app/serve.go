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
	"github.com/QwikByte/mc-server-manager/internal/master/network"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
	"github.com/QwikByte/mc-server-manager/internal/master/server"
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

	users := auth.NewService(db)
	nodes := node.NewService(db, ca, masterCert, publicAddr)
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
	httpServer := &http.Server{Addr: cfg.httpAddr, Handler: routes(users, nodes, network.NewService(db, nodes)), ReadHeaderTimeout: 10 * time.Second}

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
		"enrollment", cfg.enrollAddr, "public_enrollment", publicAddr, "ca_fingerprint", pki.Fingerprint(ca.Cert))

	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.GracefulStop()
	return errors.Join(err, httpServer.Shutdown(shutdownCtx))
}

func routes(users *auth.Service, nodes *node.Service, networks *network.Service) http.Handler {
	authHandler := auth.NewHandler(users)
	api := http.NewServeMux()
	authHandler.Register(api)
	node.NewHandler(nodes).Register(api)
	server.NewHandler(nodes, networks).Register(api)
	network.NewHandler(networks).Register(api)

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
