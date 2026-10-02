// Package runtime abstracts how Minecraft servers are executed on a node, so that
// further runtimes (e.g. plain processes) can be added next to Docker.
package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/logging"
)

// idPattern matches server IDs, which also name directories and containers.
var idPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)

// NewID returns a random server ID.
func NewID() string { return strings.ToLower(rand.Text()) }

// ValidID reports whether id is a well-formed server ID.
func ValidID(id string) bool { return idPattern.MatchString(id) }

var (
	// ErrNotFound is returned for operations on unknown servers.
	ErrNotFound = errors.New("server not found")
	// ErrNotRunning is returned for console commands to a stopped server.
	ErrNotRunning = errors.New("server is not running")
	// ErrUnsupported is returned for operations the server type does not support.
	ErrUnsupported = errors.New("not supported for this server type")
	// ErrNotReady is returned when a server that is starting can't save its worlds yet.
	ErrNotReady = errors.New("the server is starting")
)

// Spec describes a server.
type Spec struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     mcsmv1.ServerType `json:"type"`
	Version  string            `json:"version"`
	MemoryMB uint32            `json:"memoryMb"`
	Port     uint32            `json:"port"`
	// BehindProxy makes a game server accept only players forwarded by its proxy.
	BehindProxy bool `json:"behindProxy,omitempty"`
	// Storage is the storage location of the server's data; empty means the default.
	Storage string `json:"storage,omitempty"`
	// Java selects the Java version of a game server, e.g. "21"; empty means the newest.
	Java          string               `json:"java,omitempty"`
	RestartPolicy mcsmv1.RestartPolicy `json:"restartPolicy,omitempty"`
	AikarFlags    bool                 `json:"aikarFlags,omitempty"`
	JVMOptions    []string             `json:"jvmOptions,omitempty"`
	// CPUMillis limits the CPU time in thousandths of a core; 0 means no limit.
	CPUMillis uint32 `json:"cpuMillis,omitempty"`
}

// Server is a server managed by a runtime.
type Server struct {
	Spec
	State mcsmv1.ServerState
}

// Network is the role of a server in a network behind a Velocity proxy.
type Network struct {
	// ForwardingSecret lets backends verify the players the proxy forwards. An empty
	// secret makes a game server standalone again; a proxy always needs one.
	ForwardingSecret string
	// Backends is set for the proxy, in the order players are sent to them.
	Backends []NetworkBackend
}

// NetworkBackend is a server behind the proxy, either on the same node (ServerID) or
// on another node (Address).
type NetworkBackend struct {
	Name     string
	ServerID string
	Address  string
}

// Usage is what a running server uses. CPU time and network traffic count up from the
// start of the server.
type Usage struct {
	CPUTime     time.Duration
	MemoryBytes uint64
	// MemoryLimit is the most memory the server may use; 0 if it isn't limited.
	MemoryLimit uint64
	NetRxBytes  uint64
	NetTxBytes  uint64
	// Host is the address at which the agent reaches the ports of the server, e.g. its
	// console port; empty if it can't.
	Host string
}

// Info describes the runtime and the machine it runs on.
type Info struct {
	Name        string // e.g. "docker 29.0.0"
	OS          string
	CPUs        uint32
	MemoryBytes uint64
}

// Runtime executes Minecraft servers. Servers keep running when the agent restarts.
type Runtime interface {
	Info(ctx context.Context) (Info, error)
	List(ctx context.Context) ([]Server, error)
	Create(ctx context.Context, spec Spec) error
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Remove(ctx context.Context, id string) error
	// Logs yields the last tail console lines, then follows the console until the
	// server stops or ctx is cancelled.
	Logs(ctx context.Context, id string, tail int) iter.Seq2[string, error]
	// SendCommand runs a console command and returns its output.
	SendCommand(ctx context.Context, id, command string) (string, error)
	// Usage returns what a running server uses, or ErrNotRunning.
	Usage(ctx context.Context, id string) (Usage, error)
	// Update replaces the settings of a server, keeping its type, data and network role.
	// A running server restarts.
	Update(ctx context.Context, spec Spec) error
	// Restart stops a server gracefully and starts it again.
	Restart(ctx context.Context, id string) error
	// Configure gives a server its role in a network and restarts it if it runs.
	Configure(ctx context.Context, id string, network Network) error
	// Data opens the data directory of a server; the caller closes it.
	Data(ctx context.Context, id string) (*datadir.Dir, error)
	// Duplicate creates a stopped server with a copy of the data of the server from, in
	// the same storage location. The copy is standalone: whatever ties the original to a
	// network, such as the forwarding secret, is reset or left out.
	Duplicate(ctx context.Context, from string, spec Spec) error
}

// Find returns the server with the given ID, or ErrNotFound.
func Find(ctx context.Context, rt Runtime, id string) (Server, error) {
	servers, err := rt.List(ctx)
	if err != nil {
		return Server{}, err
	}
	i := slices.IndexFunc(servers, func(s Server) bool { return s.ID == id })
	if i < 0 {
		return Server{}, ErrNotFound
	}
	return servers[i], nil
}

// PauseSaving makes a running game server write its worlds to disk and stop saving, so
// that its data can be copied consistently while players stay connected. resume turns
// saving on again. Stopped servers and proxies need nothing.
func PauseSaving(ctx context.Context, rt Runtime, srv Server) (resume func(), err error) {
	if srv.State == mcsmv1.ServerState_SERVER_STATE_STOPPED || srv.Type.Proxy() {
		return func() {}, nil
	}
	if _, err := rt.SendCommand(ctx, srv.ID, "save-off"); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	resume = func() {
		if _, err := rt.SendCommand(context.WithoutCancel(ctx), srv.ID, "save-on"); err != nil {
			slog.Warn("Can't turn saving on again", logging.Servers, logging.KeyServer, srv.ID, "err", err)
		}
	}
	if _, err := rt.SendCommand(ctx, srv.ID, "save-all flush"); err != nil {
		resume()
		return nil, err
	}
	return resume, nil
}
