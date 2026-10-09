package workflow

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

// admin saves workflows with every permission.
var admin = Author{Grants: access.Admin()}

const (
	serverA = "aaaaaaaaaaaaaaaaaaaaaaaaaa"
	serverB = "bbbbbbbbbbbbbbbbbbbbbbbbbb"
	missing = "cccccccccccccccccccccccccc"
)

func target(node, id string) schedule.Target {
	return schedule.Target{Kind: schedule.KindServer, NodeID: node, ServerID: id}
}

// targetsOf returns the targets in the settings of a step.
func targetsOf(t *testing.T, st Step) []schedule.Target {
	t.Helper()
	var s Servers
	if err := json.Unmarshal(st.With, &s); err != nil {
		t.Fatal(err)
	}
	return s.Targets
}

// References follow servers that move and leave with those that are deleted, without ever
// widening a trigger or a step: a trigger whose servers are all gone is removed, a step without
// servers is turned off.
func TestLinks(t *testing.T) {
	s := testService(t)
	d := Draft{Name: "Linked", Enabled: true, Definition: Definition{
		Triggers: []Trigger{
			{Kind: OnServer, On: "started", Targets: []schedule.Target{target("n1", serverA)}},
			{Kind: OnMetric, Measure: "players", Value: 1, Targets: []schedule.Target{target("n1", serverA), target("n1", serverB)}},
		},
		Steps: []Step{
			step("restart_a", "restart", Servers{Targets: []schedule.Target{target("n1", serverA)}}),
			step("restart_both", "restart", Servers{Targets: []schedule.Target{target("n1", serverA), target("n1", serverB)}}),
			step("start_from", "start", Servers{Targets: []schedule.Target{target("n1", serverA)}, From: "{{trigger.server}}"}),
		},
	}}
	wf, err := s.Create(t.Context(), d, admin)
	if err != nil {
		t.Fatal(err)
	}

	// A server that moves stays a target at its new place.
	if err := s.Move(t.Context(), serverB, "n1", "n2"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.get(t.Context(), wf.ID)
	if ts := targetsOf(t, got.Steps[1]); !slices.Equal(ts, []schedule.Target{target("n1", serverA), target("n2", serverB)}) {
		t.Fatalf("after the move: %+v", ts)
	}

	// A deleted server leaves all targets.
	if err := s.Forget(t.Context(), "n1", serverA); err != nil {
		t.Fatal(err)
	}
	got, _ = s.get(t.Context(), wf.ID)
	if len(got.Triggers) != 1 || got.Triggers[0].Kind != OnMetric || !slices.Equal(got.Triggers[0].Targets, []schedule.Target{target("n2", serverB)}) {
		t.Fatalf("triggers = %+v", got.Triggers)
	}
	switch {
	case !got.Steps[0].Disabled || len(targetsOf(t, got.Steps[0])) != 0:
		t.Fatalf("the step without servers = %+v", got.Steps[0])
	case got.Steps[1].Disabled || !slices.Equal(targetsOf(t, got.Steps[1]), []schedule.Target{target("n2", serverB)}):
		t.Fatalf("the step with a server left = %+v", got.Steps[1])
	case got.Steps[2].Disabled:
		t.Fatalf("the step with servers from data was turned off")
	}
	// The workflow, with a step turned off that is incomplete now, still runs and can be saved.
	if !s.active[wf.ID].wf.Enabled {
		t.Fatal("the workflow no longer runs")
	}
	if _, err := s.Update(t.Context(), wf.ID, Draft{Name: got.Name, Enabled: true, Definition: got.Definition}, admin); err != nil {
		t.Fatalf("saving it again: %v", err)
	}
}

// What a step acts on that is deleted turns it off, and targets of nodes and networks that are
// gone are removed, whatever deleted them.
func TestPrune(t *testing.T) {
	s := testService(t)
	callee, err := s.Create(t.Context(), Draft{Name: "Callee", Definition: Definition{Steps: []Step{step("note", "log", logSettings{Message: "hi"})}}}, admin)
	if err != nil {
		t.Fatal(err)
	}
	caller, err := s.Create(t.Context(), Draft{Name: "Caller", Enabled: true, Definition: Definition{
		Triggers: []Trigger{{Kind: OnEvent, Level: "warn", Targets: []schedule.Target{{Kind: schedule.KindNetwork, Value: missing}}}},
		Steps: []Step{
			step("call", "call", callSettings{Workflow: callee.ID}),
			step("notify", "notify", notifySettings{Channel: missing, Message: "hi"}),
			step("maintenance", "maintenance", maintenanceSettings{Network: missing, Enabled: true}),
			step("start", "start", Servers{Targets: []schedule.Target{target("removed", serverA)}}),
		},
	}}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(t.Context(), Draft{Name: "Broken", Definition: Definition{Steps: []Step{step("call", "call", callSettings{Workflow: missing})}}}, admin); err == nil {
		t.Fatal("saved a call of a workflow that doesn't exist")
	}
	if err := s.Prune(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.get(t.Context(), caller.ID)
	off := map[string]bool{}
	for _, st := range got.Steps {
		off[st.ID] = st.Disabled
	}
	if len(got.Triggers) != 0 || off["call"] || !off["notify"] || !off["maintenance"] || !off["start"] {
		t.Fatalf("triggers = %+v, turned off = %v", got.Triggers, off)
	}
	if err := s.Delete(t.Context(), callee.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.get(t.Context(), caller.ID); !got.Steps[0].Disabled {
		t.Fatal("the call of a deleted workflow is still on")
	}
}
