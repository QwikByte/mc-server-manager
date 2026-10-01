// Package runtime abstracts how Minecraft servers are executed on a node, so that
// further runtimes (e.g. plain processes) can be added next to Docker.
package runtime

import (
	"context"
	"errors"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
)

// ErrNotFound is returned for operations on unknown servers.
var ErrNotFound = errors.New("server not found")

// Spec describes a server.
type Spec struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     mcsmv1.ServerType `json:"type"`
	Version  string            `json:"version"`
	MemoryMB uint32            `json:"memoryMb"`
	Port     uint32            `json:"port"`
}

// Server is a server managed by a runtime.
type Server struct {
	Spec
	State mcsmv1.ServerState
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
}
