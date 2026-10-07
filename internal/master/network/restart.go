package network

import (
	"context"
	"maps"
	"slices"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// RestartServers restarts running servers so that they load what changed, e.g. their plugins:
// the game servers of each network batch at a time with RollingRestart, so that it stays open,
// and the others at once, up to operation.PerNode of a node at a time. It returns the error of
// each server. Restarts can't be cancelled, and each takes the time it needs.
func (s *Service) RestartServers(ctx context.Context, servers []tag.Server, batch int) []error {
	ctx = context.WithoutCancel(ctx)
	errs := make([]error, len(servers))
	pending := map[Ref]int{}
	for i, srv := range servers {
		pending[Ref(srv)] = i
	}
	networks, err := s.List(ctx)
	if err != nil {
		for i := range errs {
			errs[i] = err
		}
		return errs
	}
	for _, n := range networks {
		var only []Ref
		for _, b := range n.Backends {
			if _, ok := pending[b.Ref]; ok {
				only = append(only, b.Ref)
			}
		}
		if len(only) == 0 {
			continue
		}
		err := s.RollingRestart(ctx, n, batch, only...)
		for _, ref := range only {
			errs[pending[ref]] = err
			delete(pending, ref)
		}
	}
	rest := slices.Sorted(maps.Values(pending))
	nodes := make([]string, len(rest))
	for j, i := range rest {
		nodes[j] = servers[i].NodeID
	}
	operation.Each(ctx, nodes, func(ctx context.Context, j int) {
		srv := servers[rest[j]]
		ctx, cancel := context.WithTimeout(ctx, configureTimeout)
		defer cancel()
		conn, err := s.nodes.Conn(ctx, srv.NodeID)
		if err == nil {
			_, err = noryxv1.NewServerServiceClient(conn).RestartServer(ctx, &noryxv1.RestartServerRequest{Id: srv.ServerID})
		}
		errs[rest[j]] = err
	}, nil)
	return errs
}
