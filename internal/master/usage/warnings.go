package usage

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"math"
	"slices"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/node"
)

const (
	// gap is how long a measure may go unmeasured, e.g. while its node is offline or the
	// console of a server doesn't answer, before a crossing that wasn't warned about yet
	// starts over.
	gap = 5 * time.Minute
	// maxLocations is the most storage locations of a node that are checked, far more than a
	// node has, so that a compromised agent can't fill the master's memory.
	maxLocations = 32
)

// key names a measure of a node, of one of its servers, or of one of its storage locations.
type key struct{ node, server, measure, storage string }

// reading is a value of a measure in a sample; known is false if the agent didn't tell it,
// e.g. the ticks per second of a server that is starting.
type reading struct {
	key
	threshold Threshold
	value     float64
	known     bool
}

// crossing is a measure beyond its threshold since a time, and whether it was warned about.
type crossing struct {
	since, seen time.Time // seen is when it was last measured beyond its threshold
	value       float64
	threshold   Threshold
	warned      bool
}

// subject is a measure of servers or of nodes.
type subject struct {
	server  bool
	measure string
}

// messages are what the log says when a measure crossed its threshold for its duration, and
// when it is fine again.
var messages = map[subject][2]string{
	{true, CPU}:      {"A server uses too much CPU", "A server uses less CPU again"},
	{true, Memory}:   {"A server is running out of memory", "A server has enough memory again"},
	{true, TPS}:      {"A server ticks too slowly", "A server ticks fast enough again"},
	{false, CPU}:     {"A node uses too much CPU", "A node uses less CPU again"},
	{false, Memory}:  {"A node is running out of memory", "A node has enough memory again"},
	{false, Storage}: {"A storage location of a node is almost full", "A storage location of a node has enough space again"},
}

// checkNode checks the sample of a node and of its recorded servers against their thresholds.
func (s *Store) checkNode(ctx context.Context, nodeID string, at time.Time, stats *noryxv1.GetStatsResponse, th effective) {
	var storage []*noryxv1.StorageLocation
	var err error
	if !th.of(nodeID, "")[Storage].Off {
		storage, err = s.storage(ctx, nodeID)
	}
	s.check(nodeID, at, readings(nodeID, stats, storage, th), err != nil)
}

// readings returns the measures of a node's sample with the thresholds that apply, leaving out
// those that are off.
func readings(nodeID string, stats *noryxv1.GetStatsResponse, storage []*noryxv1.StorageLocation, th effective) []reading {
	var rs []reading
	add := func(k key, t Threshold, value float64, known bool) {
		if !t.Off {
			rs = append(rs, reading{k, t, value, known})
		}
	}
	n, own := stats.GetNode(), th.of(nodeID, "")
	cores := float64(n.GetCpuCount()) * 1000
	add(key{nodeID, "", CPU, ""}, own[CPU], percent(float64(n.GetCpuMillis()), cores), cores > 0)
	add(key{nodeID, "", Memory, ""}, own[Memory], percent(float64(n.GetMemoryUsedBytes()), float64(n.GetMemoryTotalBytes())), n.GetMemoryTotalBytes() > 0)
	for _, l := range storage {
		total := float64(l.GetTotalBytes())
		add(key{nodeID, "", Storage, l.GetName()}, own[Storage], 100-percent(float64(l.GetFreeBytes()), total), total > 0)
	}
	for _, srv := range recorded(stats) {
		own := th.of(nodeID, srv.GetId())
		limit := cmp.Or(float64(srv.GetCpuLimitMillis()), cores)
		add(key{nodeID, srv.GetId(), CPU, ""}, own[CPU], percent(float64(srv.GetCpuMillis()), limit), limit > 0)
		add(key{nodeID, srv.GetId(), Memory, ""}, own[Memory], percent(float64(srv.GetMemoryBytes()), float64(srv.GetMemoryLimitBytes())), srv.GetMemoryLimitBytes() > 0)
		add(key{nodeID, srv.GetId(), TPS, ""}, own[TPS], srv.GetTps(), validTPS(srv.GetTps()))
	}
	return rs
}

func percent(part, whole float64) float64 {
	if whole <= 0 {
		return 0
	}
	return part / whole * 100
}

// validTPS tells whether the agent told the ticks per second, which are 20 at best.
func validTPS(tps float64) bool { return tps > 0 && tps <= 20 }

// check compares the readings of a node's sample with their thresholds. It logs a warning once
// a measure has been beyond its threshold for its duration, and an entry once it is fine
// again; nothing in between. Crossings of the node that weren't read are forgotten, e.g. of
// servers that stopped and of thresholds turned off, but those of storage locations are kept
// if keepStorage tells that they couldn't be read.
func (s *Store) check(nodeID string, at time.Time, rs []reading, keepStorage bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	read := map[key]bool{}
	for k := range s.crossings {
		if keepStorage && k.node == nodeID && k.measure == Storage {
			read[k] = true // stays as it is
		}
	}
	for _, r := range rs {
		read[r.key] = true
		c := s.crossings[r.key]
		switch {
		case !r.known:
			continue
		case !r.threshold.crossed(r.measure, r.value):
			if c != nil && c.warned {
				logChange(r, false)
			}
			delete(s.crossings, r.key)
			continue
		case c == nil || !c.warned && at.Sub(c.seen) > gap:
			c = &crossing{since: at}
			s.crossings[r.key] = c
		}
		c.seen, c.value, c.threshold = at, r.value, r.threshold
		if !c.warned && at.Sub(c.since) >= time.Duration(r.threshold.Minutes)*time.Minute {
			c.warned = true
			logChange(r, true)
		}
	}
	maps.DeleteFunc(s.crossings, func(k key, _ *crossing) bool { return k.node == nodeID && !read[k] })
}

// logChange logs that a measure crossed its threshold for its duration, or that it is fine again.
func logChange(r reading, crossed bool) {
	attrs := []any{logging.Usage, logging.KeyNode, r.node}
	if r.server != "" {
		attrs = append(attrs, logging.KeyServer, r.server)
	}
	if r.storage != "" {
		attrs = append(attrs, "storage", r.storage)
	}
	attrs = append(attrs, "value", round(r.value), "threshold", r.threshold.Value, "minutes", r.threshold.Minutes)
	m := messages[subject{r.server != "", r.measure}]
	if crossed {
		slog.Warn(m[0], attrs...)
	} else {
		slog.Info(m[1], attrs...)
	}
}

func round(v float64) float64 { return math.Round(v*10) / 10 }

// forget forgets the crossings that drop tells.
func (s *Store) forget(drop func(key) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	maps.DeleteFunc(s.crossings, func(k key, _ *crossing) bool { return drop(k) })
}

// storage asks the agent of a node for its storage locations, leaving out invalid and repeated
// names.
func (s *Store) storage(ctx context.Context, nodeID string) ([]*noryxv1.StorageLocation, error) {
	ctx, cancel := context.WithTimeout(ctx, statsTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	info, err := noryxv1.NewNodeServiceClient(conn).GetInfo(ctx, &noryxv1.GetInfoRequest{})
	if err != nil {
		return nil, err
	}
	var locations []*noryxv1.StorageLocation
	for _, l := range info.GetStorage() {
		if node.StorageName.MatchString(l.GetName()) && len(locations) < maxLocations &&
			!slices.ContainsFunc(locations, func(o *noryxv1.StorageLocation) bool { return o.GetName() == l.GetName() }) {
			locations = append(locations, l)
		}
	}
	return locations, nil
}

// Warning is a measure of a node, of one of its servers or of one of its storage locations
// that has been beyond its threshold for its duration.
type Warning struct {
	NodeID    string    `json:"nodeId"`
	ServerID  string    `json:"serverId,omitempty"`
	Measure   string    `json:"measure"`
	Storage   string    `json:"storage,omitempty"`
	Value     float64   `json:"value"`
	Threshold Threshold `json:"threshold"`
	Since     time.Time `json:"since"`
}

// Warnings returns the current warnings that sees allows, the oldest first. The master
// forgets them when it restarts.
func (s *Store) Warnings(sees func(nodeID, serverID string) bool) []Warning {
	s.mu.Lock()
	defer s.mu.Unlock()
	warnings := []Warning{}
	for k, c := range s.crossings {
		if c.warned && sees(k.node, k.server) {
			warnings = append(warnings, Warning{k.node, k.server, k.measure, k.storage, round(c.value), c.threshold, c.since})
		}
	}
	slices.SortFunc(warnings, func(a, b Warning) int {
		return cmp.Or(a.Since.Compare(b.Since), cmp.Compare(a.NodeID+a.ServerID+a.Measure+a.Storage, b.NodeID+b.ServerID+b.Measure+b.Storage))
	})
	return warnings
}
