// Package settings stores the settings of the master that administrators change in the
// panel, and describes the running master. Settings apply right away; what only changes
// with a restart, such as listen addresses and TLS, stays on the command line.
package settings

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
	"github.com/QwikByte/mc-server-manager/internal/pki"
)

const (
	maxSessionHours     = 7 * 24
	minJoinTokenMinutes = 5
	maxJoinTokenMinutes = 24 * 60
)

var hostname = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)

// Settings are the settings of the master.
type Settings struct {
	// EnrollAddr is the host:port join tokens tell agents to enroll at; empty means the
	// address given on the command line.
	EnrollAddr string `json:"enrollAddr"`
	// SessionHours is how long a sign-in to the panel lasts.
	SessionHours int `json:"sessionHours"`
	// JoinTokenMinutes is how long a join token for a node is valid.
	JoinTokenMinutes int `json:"joinTokenMinutes"`
	// NodeDefaults are the limits new nodes get.
	NodeDefaults node.Limits `json:"nodeDefaults"`
}

// defaults apply until the settings are changed, and to settings added later.
func defaults() Settings {
	return Settings{SessionHours: 12, JoinTokenMinutes: 60, NodeDefaults: node.Limits{MemoryReserveMB: new(uint32(1024))}}
}

// Master describes the running master. Apart from its certificate, it only changes with a restart.
type Master struct {
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
	// PanelAddr is where the panel listens; PanelTLS tells whether it serves HTTPS itself.
	PanelAddr string `json:"panelAddr"`
	PanelTLS  bool   `json:"panelTls"`
	// EnrollListenAddr is where the enrollment endpoint listens. EnrollAddr is the address
	// for join tokens given on the command line, which the settings can replace.
	EnrollListenAddr     string    `json:"enrollListenAddr"`
	EnrollAddr           string    `json:"enrollAddr"`
	CAFingerprint        string    `json:"caFingerprint"`
	CertificateExpiresAt time.Time `json:"certificateExpiresAt"`
}

type Service struct {
	db     *sql.DB
	master Master
	cert   *pki.Holder // the master's certificate, renewed while it runs

	mu      sync.Mutex // one update at a time, so the database and current agree
	current atomic.Pointer[Settings]
}

// Load reads the stored settings of the master described by master and cert.
func Load(ctx context.Context, db *sql.DB, master Master, cert *pki.Holder) (*Service, error) {
	current := defaults()
	var value []byte
	err := db.QueryRowContext(ctx, `SELECT value FROM settings`).Scan(&value)
	if err == nil {
		err = json.Unmarshal(value, &current)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	s := &Service{db: db, master: master, cert: cert}
	s.current.Store(&current)
	return s, nil
}

// Get returns the current settings.
func (s *Service) Get() Settings { return *s.current.Load() }

// Update validates and stores the settings. They apply right away.
func (s *Service) Update(ctx context.Context, next Settings) (Settings, error) {
	next.EnrollAddr = strings.TrimSpace(next.EnrollAddr)
	if err := validate(next); err != nil {
		return next, err
	}
	value, err := json.Marshal(next)
	if err != nil {
		return next, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO settings (id, value) VALUES (1, ?)
		ON CONFLICT (id) DO UPDATE SET value = excluded.value`, value); err != nil {
		return next, err
	}
	s.current.Store(&next)
	return next, nil
}

// Master describes the running master.
func (s *Service) Master() Master {
	m := s.master
	m.CertificateExpiresAt = s.cert.Get().Leaf.NotAfter
	return m
}

// EnrollAddr implements node.Config.
func (s *Service) EnrollAddr() string { return cmp.Or(s.Get().EnrollAddr, s.master.EnrollAddr) }

// JoinTokenTTL implements node.Config.
func (s *Service) JoinTokenTTL() time.Duration {
	return time.Duration(s.Get().JoinTokenMinutes) * time.Minute
}

// NodeDefaults implements node.Config.
func (s *Service) NodeDefaults() node.Limits { return s.Get().NodeDefaults }

// SessionTTL is how long new sign-ins to the panel last.
func (s *Service) SessionTTL() time.Duration { return time.Duration(s.Get().SessionHours) * time.Hour }

func validate(s Settings) error {
	switch {
	case s.EnrollAddr != "" && !validEnrollAddr(s.EnrollAddr):
		return httpapi.Errorf(http.StatusBadRequest, "Enter the enrollment address as host:port, for example panel.example.com:9443, or leave it empty.")
	case s.SessionHours < 1 || s.SessionHours > maxSessionHours:
		return httpapi.Errorf(http.StatusBadRequest, "Enter a session duration from 1 to %d hours.", maxSessionHours)
	case s.JoinTokenMinutes < minJoinTokenMinutes || s.JoinTokenMinutes > maxJoinTokenMinutes:
		return httpapi.Errorf(http.StatusBadRequest, "Enter a join token validity from %d to %d minutes.", minJoinTokenMinutes, maxJoinTokenMinutes)
	}
	return s.NodeDefaults.Validate()
}

// validEnrollAddr accepts a host name or IP address with a port, which agents dial as is.
func validEnrollAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	n, err := strconv.ParseUint(port, 10, 16)
	return err == nil && n > 0 && (hostname.MatchString(host) || net.ParseIP(host) != nil)
}
