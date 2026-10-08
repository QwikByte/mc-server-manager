package network

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"strconv"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// numbered matches the number at the end of the name of a server in a network, e.g. -2.
var numbered = regexp.MustCompile(`-[0-9]+$`)

// CheckCopy fails unless a copy of a server can join its network: the server must be a game
// server of a network that has room for another.
func (s *Service) CheckCopy(ctx context.Context, nodeID, serverID string) error {
	_, _, err := s.original(ctx, Ref{nodeID, serverID})
	return err
}

// AddCopy adds the copy of a game server on the same node to the network of the original,
// in its place: right after it among the servers, the servers players join and those of
// host names, with its settings and the next free name, e.g. lobby-2 for lobby. Then it
// configures the network.
func (s *Service) AddCopy(ctx context.Context, nodeID, originalID, copyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, i, err := s.original(ctx, Ref{nodeID, originalID})
	if err != nil {
		return err
	}
	original := current.Backends[i]
	b := original
	b.Ref, b.Name = Ref{nodeID, copyID}, copyName(original.Name, current.Backends)
	n := current
	n.Backends = slices.Insert(slices.Clone(current.Backends), i+1, b)
	n.Try = after(current.Try, original.Name, b.Name)
	n.ForcedHosts = slices.Clone(current.ForcedHosts)
	if !noryxv1.ParseServerType(n.ProxyType).Bungee() { // BungeeCord sends a host name to one server
		for j, h := range n.ForcedHosts {
			n.ForcedHosts[j].Servers = after(h.Servers, original.Name, b.Name)
		}
	}
	_, err = s.update(ctx, current, n)
	return err
}

// original returns the network of a game server, and its place among the servers.
func (s *Service) original(ctx context.Context, ref Ref) (Network, int, error) {
	n, err := s.find(ctx, ref.ServerID)
	if err != nil {
		return Network{}, 0, err
	}
	i := -1
	if n != nil {
		i = slices.IndexFunc(n.Backends, func(b Backend) bool { return b.Ref == ref })
	}
	switch {
	case i < 0:
		return Network{}, 0, httpapi.Errorf(http.StatusConflict, "Only copies of game servers of a network can join it.")
	case len(n.Backends) >= maxBackends:
		return Network{}, 0, httpapi.Errorf(http.StatusConflict, "The network %q has %d servers, the most a network can have.", n.Name, maxBackends)
	}
	return *n, i, nil
}

// copyName returns the name of a copy of a server in its network: the name of the original
// without its number, with the next free number.
func copyName(original string, existing []Backend) string {
	base := numbered.ReplaceAllString(original, "")
	for i := 2; ; i++ {
		suffix := "-" + strconv.Itoa(i)
		name := base[:min(len(base), 32-len(suffix))] + suffix
		if !slices.ContainsFunc(existing, func(b Backend) bool { return b.Name == name }) {
			return name
		}
	}
}

// after returns names with name right after original, if names has it.
func after(names []string, original, name string) []string {
	if i := slices.Index(names, original); i >= 0 {
		return slices.Insert(slices.Clone(names), i+1, name)
	}
	return names
}
