package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

// Workflows refer to servers, nodes and networks in their targets, and their steps to networks,
// notification channels and other workflows. When these are deleted or move, the references
// follow: a server that moves is a target at its new place, and what is deleted leaves the
// targets. Following never widens what a workflow does: a trigger whose servers are all gone is
// removed, as one without targets watches all servers, and a step that has nothing left to act
// on is turned off.

// links change the references of workflows.
type links struct {
	// target returns a target as it is now, and whether it is kept.
	target func(schedule.Target) (schedule.Target, bool)
	// gone tells what a step acts on that is gone, e.g. "network"; empty if nothing is.
	gone func(kind string, with map[string]any) string
}

// apply changes the references of a definition, and returns what it changed, for the log.
func (l links) apply(d *Definition) []string {
	var changes []string
	triggers := []Trigger{}
	for i, t := range d.Triggers {
		if len(t.Targets) > 0 {
			kept, changed := l.targets(t.Targets)
			switch {
			case changed && len(kept) == 0:
				changes = append(changes, fmt.Sprintf("removed trigger %d, whose servers are all gone", i+1))
				continue
			case changed:
				changes = append(changes, fmt.Sprintf("changed the servers of trigger %d", i+1))
			}
			t.Targets = kept
		}
		triggers = append(triggers, t)
	}
	d.Triggers = triggers
	walk(d.Steps, func(st *Step) {
		var with map[string]any
		if json.Unmarshal(st.With, &with) != nil {
			return
		}
		if raw, ok := with["targets"].([]any); ok && len(raw) > 0 {
			var list []schedule.Target
			b, _ := json.Marshal(raw)
			if json.Unmarshal(b, &list) != nil {
				return
			}
			if list, changed := l.targets(list); changed {
				with["targets"] = list
				st.With, _ = json.Marshal(with)
				changes = append(changes, fmt.Sprintf("changed the servers of step %s", st.label()))
				if from, _ := with["from"].(string); len(list) == 0 && strings.TrimSpace(from) == "" && !st.Disabled {
					st.Disabled = true
					changes = append(changes, fmt.Sprintf("turned off step %s, whose servers are all gone", st.label()))
				}
			}
		}
		if l.gone == nil || st.Disabled {
			return
		}
		if what := l.gone(st.Kind, with); what != "" {
			st.Disabled = true
			changes = append(changes, fmt.Sprintf("turned off step %s, whose %s is gone", st.label(), what))
		}
	})
	return changes
}

// targets returns the targets that are kept, as they are now, and whether any changed.
func (l links) targets(in []schedule.Target) ([]schedule.Target, bool) {
	out := []schedule.Target{}
	changed := false
	for _, t := range in {
		now, keep := l.target(t)
		changed = changed || !keep || now != t
		if keep {
			out = append(out, now)
		}
	}
	return out, changed
}

// relink changes the references of all workflows, and logs what changed in which.
func (s *Service) relink(ctx context.Context, l links) error {
	s.changes.Lock()
	defer s.changes.Unlock()
	list, _, err := s.load(ctx, "")
	if err != nil {
		return err
	}
	relinked := false
	for _, wf := range list {
		changes := l.apply(&wf.Definition)
		if len(changes) == 0 {
			continue
		}
		def, err := json.Marshal(wf.Definition)
		if err == nil {
			_, err = s.db.ExecContext(ctx, `UPDATE workflows SET definition = ?, updated_at = ? WHERE id = ?`, def, time.Now().Unix(), wf.ID)
		}
		if err != nil {
			return err
		}
		relinked = true
		slog.Info("A workflow follows what it refers to", logging.Workflows, "workflow", wf.Name, "workflow_id", wf.ID, "changes", strings.Join(changes, "; "))
	}
	if !relinked {
		return nil
	}
	return s.reload(ctx)
}

// Forget removes a deleted server from the targets of all workflows.
func (s *Service) Forget(ctx context.Context, nodeID, serverID string) error {
	return s.relink(ctx, links{target: func(t schedule.Target) (schedule.Target, bool) {
		return t, t.Kind != schedule.KindServer || t.NodeID != nodeID || t.ServerID != serverID
	}})
}

// Move keeps a server that moved to another node a target of all workflows, at its new place.
func (s *Service) Move(ctx context.Context, serverID, from, to string) error {
	return s.relink(ctx, links{target: func(t schedule.Target) (schedule.Target, bool) {
		if t.Kind == schedule.KindServer && t.NodeID == from && t.ServerID == serverID {
			t.NodeID = to
		}
		return t, true
	}})
}

// Prune removes the references to nodes, networks, notification channels and workflows that no
// longer exist, whatever deleted them, e.g. the removal of a node, which takes its servers and
// networks with it.
func (s *Service) Prune(ctx context.Context) error {
	exist := map[string]map[string]bool{}
	for table, query := range map[string]string{
		"nodes": `SELECT id FROM nodes`, "networks": `SELECT id FROM networks`,
		"channels": `SELECT id FROM notification_channels`, "workflows": `SELECT id FROM workflows`,
	} {
		ids, err := database.IDs(ctx, s.db, query)
		if err != nil {
			return err
		}
		exist[table] = ids
	}
	return s.relink(ctx, links{
		target: func(t schedule.Target) (schedule.Target, bool) {
			switch t.Kind {
			case schedule.KindServer:
				return t, exist["nodes"][t.NodeID]
			case schedule.KindNetwork:
				return t, exist["networks"][t.Value]
			}
			return t, true
		},
		gone: func(kind string, with map[string]any) string {
			id := func(key string) string { v, _ := with[key].(string); return v }
			switch {
			case (kind == "send" || kind == "maintenance" || kind == "rolling") && !exist["networks"][id("network")]:
				return "network"
			case kind == "notify" && !exist["channels"][id("channel")]:
				return "notification channel"
			case kind == "call" && !exist["workflows"][id("workflow")]:
				return "workflow"
			}
			return ""
		},
	})
}
