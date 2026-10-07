// Package usage keeps a history of what nodes and their servers use, from the
// measurements of the agents, and serves it to the panel along with the latest one. It warns
// when they use too much for a while, as thresholds of the settings, or of a node or server
// itself, tell.
package usage

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/node"
)

const (
	sampleInterval = time.Minute
	retention      = 7 * 24 * time.Hour
	statsTimeout   = 10 * time.Second
	// maxServers is the most servers of a node recorded per sample, far more than a node runs, so
	// that a compromised agent can't fill the database.
	maxServers = 500
)

// serverPattern matches the IDs of servers, random base32 in lowercase.
var serverPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)

// Ranges are the time spans a history covers, with the step its averages are taken over.
var Ranges = map[string]struct{ Span, Step time.Duration }{
	"day":  {24 * time.Hour, 5 * time.Minute},
	"week": {retention, 30 * time.Minute},
}

// Nodes provides the nodes and connections to their agents.
type Nodes interface {
	List(ctx context.Context) ([]node.Node, error)
	Get(ctx context.Context, id string) (node.Node, error)
	Conn(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)
}

// Config provides the settings of the master that concern usage. They can change at any time.
type Config interface {
	// Thresholds are the default thresholds of nodes and servers.
	Thresholds() Defaults
}

// Store keeps what nodes and servers used during the last week, and the measures that are
// beyond their thresholds.
type Store struct {
	db    *sql.DB
	nodes Nodes
	conf  Config

	mu        sync.Mutex
	crossings map[key]*crossing
}

func NewStore(db *sql.DB, nodes Nodes, conf Config) *Store {
	return &Store{db: db, nodes: nodes, conf: conf, crossings: map[key]*crossing{}}
}

// Run records the latest measurement of every agent each minute until ctx ends.
func (s *Store) Run(ctx context.Context) {
	t := time.NewTicker(sampleInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.sample(ctx, now)
		}
	}
}

// sample records the latest measurement of every agent, and checks it against the thresholds
// unless they can't be read.
func (s *Store) sample(ctx context.Context, now time.Time) {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		slog.Warn("Can't record the usage of the nodes", logging.Nodes, "err", err)
		return
	}
	th, thErr := s.thresholds(ctx)
	if thErr != nil {
		slog.Warn("Can't read the thresholds of usage", logging.Usage, "err", thErr)
	}
	enrolled := map[string]bool{}
	var wg sync.WaitGroup
	for _, n := range nodes {
		if n.EnrolledAt == nil {
			continue
		}
		enrolled[n.ID] = true
		wg.Go(func() {
			stats, err := s.Latest(ctx, n.ID)
			if err == nil {
				err = s.add(ctx, n.ID, now, stats)
			}
			if err != nil { // offline nodes are left out
				slog.Debug("Can't record the usage of a node", logging.Nodes, logging.KeyNode, n.ID, "err", err)
			} else if thErr == nil {
				s.checkNode(ctx, n.ID, now, stats, th)
			}
		})
	}
	wg.Wait()
	s.forget(func(k key) bool { return !enrolled[k.node] })
	if _, err := s.db.ExecContext(ctx, `DELETE FROM usage_samples WHERE time < ?`, now.Add(-retention).Unix()); err != nil {
		slog.Warn("Can't delete old usage", logging.Nodes, "err", err)
	}
}

// Latest asks the agent of a node for its latest measurement.
func (s *Store) Latest(ctx context.Context, nodeID string) (*noryxv1.GetStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, statsTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	res, err := noryxv1.NewStatsServiceClient(conn).GetStats(ctx, &noryxv1.GetStatsRequest{})
	if status.Code(err) == codes.Unimplemented {
		err = httpapi.Errorf(http.StatusNotImplemented, "Update the agent of this node to see what it uses.")
	}
	return res, err
}

// add records the node and its running servers.
func (s *Store) add(ctx context.Context, nodeID string, at time.Time, stats *noryxv1.GetStatsResponse) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	insert, err := tx.PrepareContext(ctx, `
		INSERT INTO usage_samples (time, node_id, server_id, cpu_millis, memory_bytes, net_received, net_sent, disk_bytes, players, tps)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	n := stats.GetNode()
	if _, err := insert.ExecContext(ctx, at.Unix(), nodeID, "", n.GetCpuMillis(), n.GetMemoryUsedBytes(), 0, 0, 0, nil, nil); err != nil {
		return err
	}
	for _, srv := range recorded(stats) {
		var players, tps any
		if p := srv.GetPlayers(); p != nil {
			players = p.GetOnline()
		}
		if validTPS(srv.GetTps()) {
			tps = srv.GetTps()
		}
		if _, err := insert.ExecContext(ctx, at.Unix(), nodeID, srv.GetId(), srv.GetCpuMillis(), srv.GetMemoryBytes(),
			srv.GetNetworkReceivedBytesPerSecond(), srv.GetNetworkSentBytesPerSecond(), srv.GetDiskBytes(), players, tps); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// recorded returns the running servers of a measurement with valid IDs, each once, and at most
// maxServers of them.
func recorded(stats *noryxv1.GetStatsResponse) []*noryxv1.ServerStats {
	var servers []*noryxv1.ServerStats
	seen := map[string]bool{}
	for _, srv := range stats.GetServers() {
		if srv.GetRunning() && serverPattern.MatchString(srv.GetId()) && !seen[srv.GetId()] && len(servers) < maxServers {
			seen[srv.GetId()] = true
			servers = append(servers, srv)
		}
	}
	return servers
}

// Point is what a node or server used on average during a step of a history.
type Point struct {
	Time            time.Time `json:"time"`
	CPUMillis       float64   `json:"cpuMillis"`
	MemoryBytes     float64   `json:"memoryBytes"`
	NetworkReceived float64   `json:"networkReceived"`
	NetworkSent     float64   `json:"networkSent"`
	DiskBytes       int64     `json:"diskBytes"`
	// Players is the most players during the step.
	Players *int64   `json:"players"`
	TPS     *float64 `json:"tps"`
}

// History returns the usage of a node, or of one of its servers, since span ago, with the
// averages of each step. Steps in which it didn't run are missing.
func (s *Store) History(ctx context.Context, nodeID, serverID string, span, step time.Duration) ([]Point, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT time / ?1 * ?1 AS start, AVG(cpu_millis), AVG(memory_bytes), AVG(net_received), AVG(net_sent),
			MAX(disk_bytes), MAX(players), AVG(tps)
		FROM usage_samples WHERE node_id = ?2 AND server_id = ?3 AND time >= ?4
		GROUP BY start ORDER BY start`,
		int64(step.Seconds()), nodeID, serverID, time.Now().Add(-span).Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []Point{}
	for rows.Next() {
		var p Point
		var start int64
		var players sql.NullInt64
		var tps sql.NullFloat64
		if err := rows.Scan(&start, &p.CPUMillis, &p.MemoryBytes, &p.NetworkReceived, &p.NetworkSent, &p.DiskBytes, &players, &tps); err != nil {
			return nil, err
		}
		p.Time = time.Unix(start, 0)
		if players.Valid {
			p.Players = &players.Int64
		}
		if tps.Valid {
			p.TPS = &tps.Float64
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// Forget deletes the history and the thresholds of a deleted server, and its warnings.
func (s *Store) Forget(ctx context.Context, nodeID, serverID string) error {
	s.forget(func(k key) bool { return k.node == nodeID && k.server == serverID })
	_, err := s.db.ExecContext(ctx, `DELETE FROM usage_samples WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	if err == nil {
		_, err = s.db.ExecContext(ctx, `DELETE FROM usage_thresholds WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	}
	return err
}

// Move keeps the history and the thresholds of a server that moved to another node. Its
// warnings start over there.
func (s *Store) Move(ctx context.Context, serverID, from, to string) error {
	s.forget(func(k key) bool { return k.node == from && k.server == serverID })
	_, err := s.db.ExecContext(ctx, `UPDATE usage_samples SET node_id = ? WHERE node_id = ? AND server_id = ?`, to, from, serverID)
	if err == nil {
		_, err = s.db.ExecContext(ctx, `UPDATE OR REPLACE usage_thresholds SET node_id = ? WHERE node_id = ? AND server_id = ?`, to, from, serverID)
	}
	return err
}
