package network

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/operation"
)

const (
	// readyTimeout is how long a server may take to run again after it restarted.
	readyTimeout = 5 * time.Minute
	pollInterval = 2 * time.Second
	// moveWait gives players a moment to move to another server before theirs restarts.
	moveWait = 3 * time.Second
	maxBatch = 50
)

var running = noryxv1.ServerState_SERVER_STATE_RUNNING

// RollingRestart restarts the running game servers of a network a few at a time, so that the
// network stays open: the players of a server move to another one first, and the next
// servers restart once these run again. The servers that players join first restart last
// and one at a time, so that players always find one.
func (s *Service) RollingRestart(ctx context.Context, n Network, batch int) error {
	if batch < 1 || batch > maxBatch {
		return httpapi.Errorf(http.StatusBadRequest, "Restart from 1 to %d servers at a time.", maxBatch)
	}
	ctx = context.WithoutCancel(ctx)
	operation.Step(ctx, "servers")
	states, err := s.states(ctx, append(refs(n.Backends), n.Proxy))
	if err != nil {
		return err
	}
	var first, last []Backend
	for _, b := range n.Backends {
		switch {
		case states[b.Ref] != running:
		case slices.Contains(n.Try, b.Name):
			last = append(last, b)
		default:
			first = append(first, b)
		}
	}
	if len(first)+len(last) == 0 {
		return httpapi.Errorf(http.StatusConflict, "No server of the network runs.")
	}
	// The server that players join first restarts last.
	slices.SortFunc(last, func(a, b Backend) int { return cmp.Compare(slices.Index(n.Try, b.Name), slices.Index(n.Try, a.Name)) })
	up := slices.Concat(first, last)
	groups := slices.Collect(slices.Chunk(first, batch))
	for _, b := range last {
		groups = append(groups, []Backend{b})
	}
	total, done := int64(len(up)), int64(0)
	operation.Count(ctx, 0, total, "servers")
	for _, group := range groups {
		if states[n.Proxy] == running {
			s.moveOff(ctx, n, group, up)
		}
		if err := s.restart(ctx, group); err != nil {
			return err
		}
		done += int64(len(group))
		operation.Count(ctx, done, total, "servers")
	}
	return nil
}

// moveOff sends the players of servers to another running server through the proxy: to the
// first that players join, if it can, and gives them a moment to move.
func (s *Service) moveOff(ctx context.Context, n Network, group, up []Backend) {
	others := slices.DeleteFunc(slices.Clone(up), func(b Backend) bool { return slices.ContainsFunc(group, b.same) })
	if len(others) == 0 {
		return // the players have nowhere to go
	}
	target := others[0].Name
	for _, name := range n.Try {
		if slices.ContainsFunc(others, func(b Backend) bool { return b.Name == name }) {
			target = name
			break
		}
	}
	players := s.players(ctx, group)
	if len(players) == 0 {
		return
	}
	conn, err := s.nodes.Conn(ctx, n.Proxy.NodeID)
	if err != nil {
		return
	}
	proxy := noryxv1.NewServerServiceClient(conn)
	for _, name := range players {
		req := &noryxv1.SendCommandRequest{Id: n.Proxy.ServerID, Command: "send " + name + " " + target}
		if _, err := proxy.SendCommand(ctx, req); err != nil {
			slog.Debug("Can't move a player before a restart", "player", name, "err", err)
		}
	}
	select {
	case <-ctx.Done():
	case <-time.After(moveWait):
	}
}

// players returns the names of the players on servers, as their agents measured them last.
func (s *Service) players(ctx context.Context, servers []Backend) []string {
	var names []string
	for _, nodeID := range nodesOf(servers) {
		ctx, cancel := context.WithTimeout(ctx, queryTimeout)
		conn, err := s.nodes.Conn(ctx, nodeID)
		var stats *noryxv1.GetStatsResponse
		if err == nil {
			stats, _ = noryxv1.NewStatsServiceClient(conn).GetStats(ctx, &noryxv1.GetStatsRequest{}) // without, players get kicked instead
		}
		cancel()
		for _, srv := range stats.GetServers() {
			if slices.ContainsFunc(servers, func(b Backend) bool { return b.Ref == Ref{nodeID, srv.GetId()} }) {
				// Names come from the server, so they must not add to the command.
				names = append(names, slices.DeleteFunc(srv.GetPlayers().GetNames(), func(n string) bool { return !noryxv1.ValidPlayerName(n) })...)
			}
		}
	}
	return names
}

// restart restarts servers at the same time and waits until they run again.
func (s *Service) restart(ctx context.Context, servers []Backend) error {
	errs := make([]error, len(servers))
	var wg sync.WaitGroup
	for i, b := range servers {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, configureTimeout)
			defer cancel()
			conn, err := s.nodes.Conn(ctx, b.NodeID)
			if err == nil {
				_, err = noryxv1.NewServerServiceClient(conn).RestartServer(ctx, &noryxv1.RestartServerRequest{Id: b.ServerID})
			}
			if err != nil {
				errs[i] = fmt.Errorf("%s: %s", b.Name, message(err))
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return httpapi.Errorf(http.StatusBadGateway, "%s", err)
	}
	err := waitFor(ctx, readyTimeout, func(ctx context.Context) (bool, error) {
		states, err := s.states(ctx, refs(servers))
		if err != nil {
			return false, err
		}
		for _, b := range servers {
			switch states[b.Ref] {
			case running:
			case noryxv1.ServerState_SERVER_STATE_STARTING:
				return false, nil
			default:
				return false, httpapi.Errorf(http.StatusBadGateway, "%s didn't start again. Its console tells why.", b.Name)
			}
		}
		return true, nil
	})
	return timedOut(err, "The servers didn't start again within 5 minutes.")
}

// states returns the states of servers, with one listing per node.
func (s *Service) states(ctx context.Context, servers []Ref) (map[Ref]noryxv1.ServerState, error) {
	states := map[Ref]noryxv1.ServerState{}
	for _, nodeID := range nodesOf(servers) {
		ctx, cancel := context.WithTimeout(ctx, queryTimeout)
		conn, err := s.nodes.Conn(ctx, nodeID)
		var res *noryxv1.ListServersResponse
		if err == nil {
			res, err = noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
		}
		cancel()
		if err != nil {
			return nil, err
		}
		for _, srv := range res.GetServers() {
			states[Ref{nodeID, srv.GetId()}] = srv.GetState()
		}
	}
	return states, nil
}

// waitFor asks done every few seconds until it is done, fails, or the time is up.
func waitFor(ctx context.Context, timeout time.Duration, done func(ctx context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		ok, err := done(ctx)
		if ok || err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// timedOut replaces the error of a wait whose time is up with a message for the operator.
func timedOut(err error, msg string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return httpapi.Errorf(http.StatusGatewayTimeout, "%s", msg)
	}
	return err
}

// nodesOf returns the nodes of servers, each once.
func nodesOf[T interface{ node() string }](servers []T) []string {
	var nodes []string
	for _, s := range servers {
		if !slices.Contains(nodes, s.node()) {
			nodes = append(nodes, s.node())
		}
	}
	return nodes
}

func (r Ref) node() string { return r.NodeID }
