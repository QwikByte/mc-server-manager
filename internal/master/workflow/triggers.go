package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/logs"
)

const (
	// pageSize entries of the log are read at once.
	pageSize = 500
	// readRetry is when the log is read again after reading it failed.
	readRetry = 10 * time.Second
	// matcherTTL is how long the servers that targets name are kept, as tags and networks change.
	matcherTTL = 30 * time.Second
)

// next returns when a schedule or interval fires after t, or the zero time for other triggers
// and schedules whose dates all passed. Intervals fire at multiples of their minutes.
func next(t Trigger, after time.Time) time.Time {
	switch t.Kind {
	case OnSchedule:
		return t.Schedule.Next(after)
	case OnInterval:
		every := time.Duration(t.Every) * time.Minute
		return after.Truncate(every).Add(every)
	}
	return time.Time{}
}

// schedule fires the schedules and intervals of enabled workflows at their times until ctx is
// done. Times missed while the master was down are skipped.
func (s *Service) schedule(ctx context.Context) {
	type due struct {
		id, kind string
		at       time.Time
	}
	for {
		now := time.Now()
		wait := time.Minute // also notices when the clock is changed
		var fire []due
		s.mu.Lock()
		for id, a := range s.active {
			for i, at := range a.next {
				switch {
				case at.IsZero():
				case at.After(now):
					wait = min(wait, at.Sub(now))
				default:
					fire = append(fire, due{id, a.wf.Triggers[i].Kind, at})
					a.next[i] = next(a.wf.Triggers[i], now)
				}
			}
		}
		s.mu.Unlock()
		for _, d := range fire {
			s.fire(d.id, d.kind, map[string]any{"kind": d.kind, "time": d.at.Format(time.RFC3339)})
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		case <-s.wake:
			timer.Stop()
		}
	}
}

// follow fires the event triggers of enabled workflows for the entries added to the log from
// now on, until ctx is done.
func (s *Service) follow(ctx context.Context) {
	ctx = access.WithGrants(ctx, access.Admin()) // triggers see every entry
	var after int64
	newest := func() error {
		list, err := s.deps.Logs.List(ctx, logs.Filter{}, false, 1)
		if err == nil && len(list) > 0 {
			after = list[0].ID
		}
		return err
	}
	for newest() != nil {
		if sleep(ctx, readRetry) != nil {
			return
		}
	}
	for {
		changed := s.deps.Logs.Changed()
		level := s.eventLevel()
		if level == "" {
			_ = newest() // no event triggers: skip what came meanwhile
		} else {
			entries, err := s.deps.Logs.List(ctx, logs.Filter{Level: level, After: after}, true, pageSize)
			switch {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Debug("Can't read the log for workflows", logging.Workflows, "err", err)
				if sleep(ctx, readRetry) != nil {
					return
				}
				continue
			}
			for _, e := range entries {
				after = e.ID
				s.event(ctx, e)
			}
			if len(entries) == pageSize {
				continue
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-changed:
		}
	}
}

// eventLevel is the least important level of the event triggers of enabled workflows, so that
// the others aren't read; empty if there are none.
func (s *Service) eventLevel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	least, found := slog.LevelError, false
	for _, a := range s.active {
		for _, t := range a.wf.Triggers {
			if level, err := logging.ParseLevel(t.Level); err == nil && t.Kind == OnEvent && a.wf.Enabled {
				least, found = min(least, level), true
			}
		}
	}
	if !found {
		return ""
	}
	return logging.LevelName(least)
}

// candidate is a trigger of a workflow that an entry or a measurement may fire.
type candidate struct {
	wf      Workflow
	index   int
	trigger Trigger
}

// candidates returns the triggers of a kind of the enabled workflows that keep returns true for.
func (s *Service) candidates(kind string, keep func(Workflow, Trigger) bool) []candidate {
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []candidate
	for _, a := range s.active {
		for i, t := range a.wf.Triggers {
			if a.wf.Enabled && t.Kind == kind && keep(a.wf, t) {
				list = append(list, candidate{a.wf, i, t})
			}
		}
	}
	return list
}

// event fires the event triggers that an entry of the log matches, each workflow once. Entries
// that a workflow wrote itself don't fire it, so that it can't keep itself running.
func (s *Service) event(ctx context.Context, e logs.Entry) {
	list := s.candidates(OnEvent, func(wf Workflow, t Trigger) bool {
		level, _ := logging.ParseLevel(t.Level)
		return e.Level >= level && (len(t.Categories) == 0 || slices.Contains(t.Categories, e.Category)) &&
			strings.Contains(strings.ToLower(e.Message), strings.ToLower(t.Contains)) && e.Attrs["workflow_id"] != wf.ID
	})
	fired := map[string]bool{}
	for _, c := range list {
		if fired[c.wf.ID] || len(c.trigger.Targets) > 0 && (e.NodeID == "" || !s.matches(ctx, c, e.NodeID, e.ServerID)) {
			continue
		}
		fired[c.wf.ID] = true
		data := map[string]any{
			"kind": OnEvent, "time": e.Time.Format(time.RFC3339), "message": e.Message, "level": logging.LevelName(e.Level),
			"category": e.Category, "user": e.User, "attrs": e.Attrs,
		}
		if e.NodeID != "" {
			data["node"] = map[string]any{"id": e.NodeID, "name": e.NodeName}
		}
		if e.ServerID != "" {
			data["server"] = map[string]any{"id": e.ServerID, "nodeId": e.NodeID, "name": e.ServerName, "node": e.NodeName}
		}
		s.fire(c.wf.ID, OnEvent, data)
	}
}

// matcher tells which servers the targets of a trigger name, as of a time.
type matcher struct {
	match   func(nodeID, serverID string) bool
	at      time.Time
	updated time.Time
}

// matches tells whether the targets of a trigger name a server, or a whole node for an empty
// server ID.
func (s *Service) matches(ctx context.Context, c candidate, nodeID, serverID string) bool {
	key := fmt.Sprint(c.wf.ID, "/", c.index)
	s.watch.mu.Lock()
	m, ok := s.watch.matchers[key]
	s.watch.mu.Unlock()
	if !ok || time.Since(m.at) > matcherTTL || !m.updated.Equal(c.wf.UpdatedAt) {
		match, err := s.deps.Targets.Matcher(ctx, c.trigger.Targets)
		if err != nil {
			slog.Debug("Can't find the servers of a trigger", logging.Workflows, "workflow", c.wf.Name, "err", err)
			return false
		}
		m = matcher{match, time.Now(), c.wf.UpdatedAt}
		s.watch.mu.Lock()
		s.watch.matchers[key] = m
		s.watch.mu.Unlock()
	}
	return m.match(nodeID, serverID)
}

// watch is what server and metric triggers know of the measurements before.
type watch struct {
	mu        sync.Mutex
	seen      map[string]map[string]*observed // the running servers of each node, by ID
	crossings map[crossKey]*crossingAt
	matchers  map[string]matcher
}

// observed is a running server at the latest measurement of its node.
type observed struct {
	// names are the players online, or nil if they were never known.
	names map[string]bool
}

// crossKey is a measure of a server beyond the value of a trigger, as the workflow was saved.
type crossKey struct {
	workflow string
	updated  time.Time
	index    int
	node     string
	server   string
}

// crossingAt is since when a measure is beyond the value of a trigger, and whether it fired.
type crossingAt struct {
	since time.Time
	fired bool
}

// event is what a measurement told about a server.
type serverEvent struct {
	on       string // started, stopped, joined or left
	serverID string
	player   string
}

// Measured fires the server and metric triggers of enabled workflows that a measurement of a
// node fires: servers that started or stopped, players who joined or left, and measures beyond
// a value for long enough. The first measurement of a node only tells what is.
func (s *Service) Measured(ctx context.Context, nodeID string, at time.Time, stats *noryxv1.GetStatsResponse, servers []*noryxv1.ServerStats) {
	events := s.watch.compare(nodeID, servers)
	if len(events) > 0 {
		list := s.candidates(OnServer, func(Workflow, Trigger) bool { return true })
		for _, ev := range events {
			for _, c := range list {
				t := c.trigger
				if t.On != ev.on || len(t.Players) > 0 && !slices.Contains(t.Players, ev.player) ||
					len(t.Targets) > 0 && !s.matches(ctx, c, nodeID, ev.serverID) {
					continue
				}
				data := s.serverData(ctx, nodeID, ev.serverID)
				data["kind"], data["on"], data["time"] = OnServer, ev.on, at.Format(time.RFC3339)
				if ev.player != "" {
					data["player"] = ev.player
				}
				s.fire(c.wf.ID, OnServer, data)
			}
		}
	}
	s.metrics(ctx, nodeID, at, stats, servers)
}

// serverData is the server of a trigger as templates see it.
func (s *Service) serverData(ctx context.Context, nodeID, serverID string) map[string]any {
	node := s.deps.Names.Node(ctx, nodeID)
	return map[string]any{
		"node":   map[string]any{"id": nodeID, "name": node},
		"server": map[string]any{"id": serverID, "nodeId": nodeID, "name": s.deps.Names.Server(ctx, nodeID, serverID), "node": node},
	}
}

// compare remembers the running servers of a node and their players, and returns what changed
// since its measurement before. A player list that misses players, as a server only named some,
// tells nothing.
func (w *watch) compare(nodeID string, servers []*noryxv1.ServerStats) []serverEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	before, known := w.seen[nodeID]
	now := map[string]*observed{}
	var events []serverEvent
	for _, srv := range servers {
		o := &observed{}
		var prev *observed
		if before != nil {
			prev = before[srv.GetId()]
		}
		if p := srv.GetPlayers(); p != nil && len(p.GetNames()) == int(p.GetOnline()) {
			o.names = map[string]bool{}
			for _, n := range p.GetNames() {
				o.names[n] = true
			}
		} else if prev != nil {
			o.names = prev.names
		}
		now[srv.GetId()] = o
		switch {
		case !known:
		case prev == nil:
			events = append(events, serverEvent{on: "started", serverID: srv.GetId()})
		case prev.names != nil && o.names != nil:
			for _, n := range slices.Sorted(maps.Keys(o.names)) {
				if !prev.names[n] {
					events = append(events, serverEvent{"joined", srv.GetId(), n})
				}
			}
			for _, n := range slices.Sorted(maps.Keys(prev.names)) {
				if !o.names[n] {
					events = append(events, serverEvent{"left", srv.GetId(), n})
				}
			}
		}
	}
	for id, prev := range before {
		if now[id] == nil {
			for _, n := range slices.Sorted(maps.Keys(prev.names)) {
				events = append(events, serverEvent{"left", id, n})
			}
			events = append(events, serverEvent{on: "stopped", serverID: id})
		}
	}
	w.seen[nodeID] = now
	return events
}

// metrics fires the metric triggers whose measure of a server of the node has been beyond their
// value for their minutes, once until it is back.
func (s *Service) metrics(ctx context.Context, nodeID string, at time.Time, stats *noryxv1.GetStatsResponse, servers []*noryxv1.ServerStats) {
	list := s.candidates(OnMetric, func(Workflow, Trigger) bool { return true })
	touched := map[crossKey]bool{}
	for _, c := range list {
		t := c.trigger
		for _, srv := range servers {
			value, ok := measure(stats, srv, t.Measure)
			beyond := ok && (t.Below && value < t.Value || !t.Below && value > t.Value)
			if !beyond || len(t.Targets) > 0 && !s.matches(ctx, c, nodeID, srv.GetId()) {
				continue
			}
			key := crossKey{c.wf.ID, c.wf.UpdatedAt, c.index, nodeID, srv.GetId()}
			touched[key] = true
			s.watch.mu.Lock()
			x := s.watch.crossings[key]
			if x == nil {
				x = &crossingAt{since: at}
				s.watch.crossings[key] = x
			}
			fire := !x.fired && at.Sub(x.since) >= time.Duration(t.Minutes)*time.Minute
			x.fired = x.fired || fire
			s.watch.mu.Unlock()
			if fire {
				data := s.serverData(ctx, nodeID, srv.GetId())
				data["kind"], data["time"], data["measure"], data["value"] = OnMetric, at.Format(time.RFC3339), t.Measure, value
				s.fire(c.wf.ID, OnMetric, data)
			}
		}
	}
	s.watch.mu.Lock()
	maps.DeleteFunc(s.watch.crossings, func(k crossKey, _ *crossingAt) bool { return k.node == nodeID && !touched[k] })
	s.watch.mu.Unlock()
}
