package logs

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/time/rate"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/logging"
)

const (
	watchEvery  = 10 * time.Second // how soon new and removed nodes are noticed
	retryAfter  = 10 * time.Second
	dropsReport = time.Minute
	// An agent may add agentRate entries per second on average, so that a compromised one
	// can't fill the database.
	agentRate  = 50
	agentBurst = 5000
)

// Collect reads the logs of the enrolled nodes' agents into the store until ctx is done.
// Each agent's log continues where it was last read, also after a restart of the master.
func (s *Store) Collect(ctx context.Context, nodes Nodes) {
	running := map[string]context.CancelFunc{}
	ticker := time.NewTicker(watchEvery)
	defer ticker.Stop()
	for {
		if list, err := nodes.List(ctx); err == nil {
			enrolled := map[string]bool{}
			for _, n := range list {
				if n.EnrolledAt == nil {
					continue
				}
				enrolled[n.ID] = true
				if running[n.ID] == nil {
					follow, cancel := context.WithCancel(ctx)
					running[n.ID] = cancel
					go s.follow(follow, nodes, n.ID)
				}
			}
			for id, cancel := range running {
				if !enrolled[id] {
					cancel()
					delete(running, id)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// follow reads the log of a node's agent, and connects again whenever it is lost.
func (s *Store) follow(ctx context.Context, nodes Nodes, nodeID string) {
	limiter := rate.NewLimiter(agentRate, agentBurst)
	var reported time.Time
	for {
		err := s.read(ctx, nodes, nodeID, limiter, &reported)
		slog.Debug("Reading the log of an agent stopped", logging.Nodes, logging.KeyNode, nodeID, "err", err)
		timer := time.NewTimer(retryAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Store) read(ctx context.Context, nodes Nodes, nodeID string, limiter *rate.Limiter, reported *time.Time) error {
	boot, seq, err := s.position(ctx, nodeID)
	if err != nil {
		return err
	}
	conn, err := nodes.Conn(ctx, nodeID)
	if err != nil {
		return err
	}
	stream, err := mcsmv1.NewLogServiceClient(conn).ReadLog(ctx, &mcsmv1.ReadLogRequest{Boot: boot, After: seq, Follow: true})
	if err != nil {
		return err
	}
	for {
		res, err := stream.Recv()
		if err != nil {
			return err
		}
		entries := res.GetEntries()
		if len(entries) == 0 {
			continue
		}
		batch, now, dropped := make([]Entry, 0, len(entries)), time.Now(), 0
		for _, e := range entries {
			if !limiter.Allow() {
				dropped++
				continue
			}
			at := time.Unix(0, e.GetTimeUnixNano())
			if at.After(now) { // an agent's clock can't put entries ahead of the others
				at = now
			}
			// The agent decides what its entries say, but not whom they are about: only its
			// node and the servers on it. Users are only known to the master.
			batch = append(batch, Entry{
				Time: at, Level: slog.Level(e.GetLevel()), Source: FromAgent, Category: e.GetCategory(), Message: e.GetMessage(),
				NodeID: nodeID, ServerID: e.GetServerId(), Attrs: e.GetAttrs(),
			})
		}
		if dropped > 0 && time.Since(*reported) >= dropsReport {
			*reported = time.Now()
			slog.Warn("Dropped log entries of an agent that logs too much", logging.Nodes, logging.KeyNode, nodeID, "entries", dropped)
		}
		last := entries[len(entries)-1]
		if err := s.write(ctx, batch, &cursor{nodeID, last.GetBoot(), last.GetSeq()}); err != nil {
			return err
		}
	}
}
