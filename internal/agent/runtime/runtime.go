// Package runtime abstracts how Minecraft servers are executed on a node, so that
// further runtimes (e.g. plain processes) can be added next to Docker.
package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"iter"
	"regexp"
	"strings"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
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
	// Restart stops a server gracefully and starts it again.
	Restart(ctx context.Context, id string) error
	// Configure gives a server its role in a network and restarts it if it runs.
	Configure(ctx context.Context, id string, network Network) error
	// Data opens the data directory of a server; the caller closes it.
	Data(ctx context.Context, id string) (*datadir.Dir, error)
}
