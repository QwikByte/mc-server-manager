// Package settings stores the settings of the master that administrators change in the
// panel, and describes the running master. Settings apply right away, except the panel's
// address and HTTPS, which apply when the master starts again. The enrollment endpoint's
// listen address stays on the command line.
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
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/https"
	"github.com/QwikByte/noryx/internal/master/node"
	"github.com/QwikByte/noryx/internal/master/usage"
	"github.com/QwikByte/noryx/internal/pki"
)

const (
	maxSessionHours     = 7 * 24
	minJoinTokenMinutes = 5
	maxJoinTokenMinutes = 24 * 60
	maxLogDays          = 365
	minLogSizeMB        = 100
	maxLogSizeMB        = 100 << 10 // 100 GiB
	maxMFAGroups        = 100
)

var (
	hostname = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)
	groupID  = regexp.MustCompile(`^[a-z0-9]{1,64}$`)
)

// Settings are the settings of the master.
type Settings struct {
	// PanelAddr is the IP:port the panel listens at from the next start of the master;
	// empty means the address given on the command line.
	PanelAddr string `json:"panelAddr"`
	// PanelHTTPS is the certificate the panel serves from the next start of the master, unless
	// the command line gives one: https.SelfSigned, https.LetsEncrypt, or empty for plain HTTP,
	// e.g. behind a reverse proxy.
	PanelHTTPS string `json:"panelHttps"`
	// PanelDomain is the panel's domain name, which Let's Encrypt certifies and the self-signed
	// certificate includes.
	PanelDomain string `json:"panelDomain"`
	// EnrollAddr is the host:port join tokens tell agents to enroll at; empty means the
	// address given on the command line.
	EnrollAddr string `json:"enrollAddr"`
	// SessionHours is how long a sign-in to the panel lasts.
	SessionHours int `json:"sessionHours"`
	// JoinTokenMinutes is how long a join token for a node is valid.
	JoinTokenMinutes int `json:"joinTokenMinutes"`
	// NodeDefaults are the limits new nodes get.
	NodeDefaults node.Limits `json:"nodeDefaults"`
	// LogDays is how long log entries are kept.
	LogDays int `json:"logDays"`
	// LogSizeMB is how many MiB the log may take at most; the oldest entries beyond it are
	// deleted before their time, so that agents can't fill the disk.
	LogSizeMB int `json:"logSizeMb"`
	// CheckUpdates makes the master look for new releases, which administrators can install.
	CheckUpdates bool `json:"checkUpdates"`
	// RequireMFA tells who has to use two-factor authentication.
	RequireMFA MFARequirement `json:"requireMfa"`
	// Thresholds tell when the usage of nodes and servers warns, unless they have their own.
	Thresholds usage.Defaults `json:"thresholds"`
}

// MFARequirement tells who has to use two-factor authentication: all users, or the members
// of the groups it names by their IDs. Groups that don't exist (any more) have no members.
type MFARequirement struct {
	All    bool     `json:"all"`
	Groups []string `json:"groups"`
}

// defaults apply until the settings are changed, and to settings added later.
func defaults() Settings {
	return Settings{
		SessionHours: 12, JoinTokenMinutes: 60, NodeDefaults: node.Limits{MemoryReserveMB: new(uint32(1024))}, LogDays: 30, LogSizeMB: 2048, CheckUpdates: true,
		RequireMFA: MFARequirement{Groups: []string{}}, Thresholds: usage.DefaultThresholds(),
	}
}

// Master describes the running master. Apart from its certificates, it only changes with a restart.
type Master struct {
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
	// PanelAddr is where the panel listens. PanelHTTPS is the certificate it serves: one of
	// https.SelfSigned, https.LetsEncrypt and https.Files, or none behind a reverse proxy.
	// PanelDomain is the domain it was set up with, PanelCertificate describes the certificate.
	PanelAddr        string             `json:"panelAddr"`
	PanelHTTPS       string             `json:"panelHttps"`
	PanelDomain      string             `json:"panelDomain"`
	PanelCertificate *https.Certificate `json:"panelCertificate,omitempty"`
	// PanelDefaultAddr is the panel's address given on the command line, where it listens
	// while the settings name none. PanelAddrError tells why it listens there although
	// they name one.
	PanelDefaultAddr string `json:"panelDefaultAddr"`
	PanelAddrError   string `json:"panelAddrError,omitempty"`
	// EnrollListenAddr is where the enrollment endpoint listens. EnrollAddr is the address
	// for join tokens given on the command line, which the settings can replace.
	EnrollListenAddr     string    `json:"enrollListenAddr"`
	EnrollAddr           string    `json:"enrollAddr"`
	CAFingerprint        string    `json:"caFingerprint"`
	CertificateExpiresAt time.Time `json:"certificateExpiresAt"`
	// Restartable tells whether administrators can restart the master from the panel.
	Restartable bool `json:"restartable"`
}

type Service struct {
	db        *sql.DB
	master    Master
	cert      *pki.Holder   // the master's certificate, renewed while it runs
	panelCert *https.Server // nil unless the panel serves a certificate of the settings

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

// Get returns a copy of the current settings, also of the limits and thresholds they point to,
// so that changing it, e.g. by decoding a request into it, leaves the current settings as they are.
func (s *Service) Get() Settings {
	st := *s.current.Load()
	l := &st.NodeDefaults
	l.PortMin, l.PortMax, l.MemoryReserveMB = clone(l.PortMin), clone(l.PortMax), clone(l.MemoryReserveMB)
	st.RequireMFA.Groups = slices.Clone(st.RequireMFA.Groups)
	st.Thresholds = st.Thresholds.Clone()
	return st
}

func clone[T any](p *T) *T {
	if p == nil {
		return nil
	}
	return new(*p)
}

// Update validates and stores the settings. They apply right away, the panel's address and
// HTTPS when the master starts again.
func (s *Service) Update(ctx context.Context, next Settings) (Settings, error) {
	next.EnrollAddr, next.PanelAddr = strings.TrimSpace(next.EnrollAddr), strings.TrimSpace(next.PanelAddr)
	next.PanelDomain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(next.PanelDomain), "."))
	if next.RequireMFA.All || next.RequireMFA.Groups == nil {
		next.RequireMFA.Groups = []string{}
	}
	slices.Sort(next.RequireMFA.Groups)
	next.RequireMFA.Groups = slices.Compact(next.RequireMFA.Groups)
	err := validate(next, cmp.Or(next.PanelAddr, s.master.PanelDefaultAddr))
	if err == nil && next.PanelAddr != "" && next.PanelAddr != s.Get().PanelAddr {
		err = s.checkPanelAddr(next.PanelAddr)
	}
	if err != nil {
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

// ListenPanel listens at the panel's address from the settings, or else at the one from
// the command line: while the settings name none, or if the master can't listen there,
// e.g. as another program took the port, so that a wrong address can't lock administrators
// out. Call it before anything reads the description of the master.
func (s *Service) ListenPanel() (net.Listener, error) {
	addr := cmp.Or(s.Get().PanelAddr, s.master.PanelDefaultAddr)
	ln, err := net.Listen("tcp", addr)
	if err != nil && addr != s.master.PanelDefaultAddr {
		s.master.PanelAddrError = err.Error()
		addr = s.master.PanelDefaultAddr
		ln, err = net.Listen("tcp", addr)
	}
	s.master.PanelAddr = addr
	return ln, err
}

// PanelHTTPS prepares in dir the certificates the panel serves as the settings say, unless the
// command line gives one, or returns nil for plain HTTP. Call it before anything reads the
// description of the master.
func (s *Service) PanelHTTPS(dir string) (*https.Server, error) {
	if s.master.PanelHTTPS == https.Files || s.Get().PanelHTTPS == "" {
		return nil, nil
	}
	enrollHost, _, _ := net.SplitHostPort(s.EnrollAddr())
	srv, err := https.New(s.Get().PanelHTTPS, s.Get().PanelDomain, []string{enrollHost}, dir)
	if err != nil {
		return nil, err
	}
	s.master.PanelHTTPS, s.master.PanelDomain, s.panelCert = s.Get().PanelHTTPS, s.Get().PanelDomain, srv
	return srv, nil
}

// checkPanelAddr tells whether the master can listen at addr when it starts again. The port
// the panel listens at now is only free then, so for it only the IP address is checked.
func (s *Service) checkPanelAddr(addr string) error {
	host, port, _ := net.SplitHostPort(addr)
	if _, current, _ := net.SplitHostPort(s.master.PanelAddr); port == current {
		port = "0"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		var op *net.OpError
		if errors.As(err, &op) {
			err = op.Err
		}
		return httpapi.Errorf(http.StatusBadRequest, "The panel can't listen at %s: %v.", addr, err)
	}
	return ln.Close()
}

// Master describes the running master.
func (s *Service) Master() Master {
	m := s.master
	m.CertificateExpiresAt = s.cert.Get().Leaf.NotAfter
	if s.panelCert != nil {
		m.PanelCertificate = new(s.panelCert.Certificate())
	}
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

// LogRetention implements logs.Config.
func (s *Service) LogRetention() time.Duration {
	return time.Duration(s.Get().LogDays) * 24 * time.Hour
}

// LogMaxSize implements logs.Config.
func (s *Service) LogMaxSize() int64 { return int64(s.Get().LogSizeMB) << 20 }

// CheckUpdates implements update.Config.
func (s *Service) CheckUpdates() bool { return s.Get().CheckUpdates }

// Thresholds implements usage.Config.
func (s *Service) Thresholds() usage.Defaults { return s.Get().Thresholds }

// SessionTTL is how long new sign-ins to the panel last.
func (s *Service) SessionTTL() time.Duration { return time.Duration(s.Get().SessionHours) * time.Hour }

// validate checks the settings; panelAddr is where the panel listens with them.
func validate(s Settings, panelAddr string) error {
	panelIP, panelPort, _ := net.SplitHostPort(panelAddr)
	switch {
	case s.PanelAddr != "" && !validAddr(s.PanelAddr, func(host string) bool { return net.ParseIP(host) != nil }):
		return httpapi.Errorf(http.StatusBadRequest, "Enter the panel's address as IP address and port, for example 0.0.0.0:8080, or leave it empty.")
	case s.PanelHTTPS != "" && s.PanelHTTPS != https.SelfSigned && s.PanelHTTPS != https.LetsEncrypt:
		return httpapi.Errorf(http.StatusBadRequest, "Choose a self-signed certificate, Let's Encrypt or none for the panel.")
	case s.PanelDomain != "" && (!hostname.MatchString(s.PanelDomain) || !strings.Contains(s.PanelDomain, ".") || net.ParseIP(s.PanelDomain) != nil):
		return httpapi.Errorf(http.StatusBadRequest, "Enter the panel's domain name, for example panel.example.com, or leave it empty.")
	case s.PanelHTTPS == https.LetsEncrypt && s.PanelDomain == "":
		return httpapi.Errorf(http.StatusBadRequest, "Let's Encrypt needs the domain name of the panel.")
	case s.PanelHTTPS == https.LetsEncrypt && net.ParseIP(panelIP).IsLoopback():
		return httpapi.Errorf(http.StatusBadRequest, "Let's Encrypt has to reach the panel from the internet: let it listen at another address than %s, e.g. 0.0.0.0:443.", panelAddr)
	case s.PanelHTTPS == https.LetsEncrypt && panelPort == "80":
		return httpapi.Errorf(http.StatusBadRequest, "With Let's Encrypt, port 80 sends browsers to HTTPS. Let the panel listen at another port, e.g. 443.")
	case s.EnrollAddr != "" && !validAddr(s.EnrollAddr, func(host string) bool { return hostname.MatchString(host) || net.ParseIP(host) != nil }):
		return httpapi.Errorf(http.StatusBadRequest, "Enter the enrollment address as host:port, for example panel.example.com:9443, or leave it empty.")
	case s.SessionHours < 1 || s.SessionHours > maxSessionHours:
		return httpapi.Errorf(http.StatusBadRequest, "Enter a session duration from 1 to %d hours.", maxSessionHours)
	case s.JoinTokenMinutes < minJoinTokenMinutes || s.JoinTokenMinutes > maxJoinTokenMinutes:
		return httpapi.Errorf(http.StatusBadRequest, "Enter a join token validity from %d to %d minutes.", minJoinTokenMinutes, maxJoinTokenMinutes)
	case s.LogDays < 1 || s.LogDays > maxLogDays:
		return httpapi.Errorf(http.StatusBadRequest, "Enter how long log entries are kept, from 1 to %d days.", maxLogDays)
	case s.LogSizeMB < minLogSizeMB || s.LogSizeMB > maxLogSizeMB:
		return httpapi.Errorf(http.StatusBadRequest, "Enter how large the log may grow, from %d to %d MiB.", minLogSizeMB, maxLogSizeMB)
	case len(s.RequireMFA.Groups) > maxMFAGroups || slices.ContainsFunc(s.RequireMFA.Groups, func(id string) bool { return !groupID.MatchString(id) }):
		return httpapi.Errorf(http.StatusBadRequest, "Choose up to %d groups that have to use two-factor authentication.", maxMFAGroups)
	}
	return cmp.Or(s.NodeDefaults.Validate(), s.Thresholds.Validate())
}

// validAddr accepts host:port with a port from 1 to 65535 and a host that validHost accepts.
func validAddr(addr string, validHost func(string) bool) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	n, err := strconv.ParseUint(port, 10, 16)
	return err == nil && n > 0 && validHost(host)
}
