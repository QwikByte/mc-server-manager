package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/node"
)

func TestNext(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) time.Time {
		parsed, err := time.ParseInLocation(time.DateTime, s, berlin)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	daily := Schedule{Times: []string{"04:00", "16:30"}, TimeZone: "Europe/Berlin"}
	weekend := Schedule{Days: []time.Weekday{time.Saturday, time.Sunday}, Times: []string{"02:30"}, TimeZone: "Europe/Berlin"}
	for _, tc := range []struct {
		schedule Schedule
		after    string
		want     string
	}{
		{daily, "2026-10-02 03:59:59", "2026-10-02 04:00:00"},
		{daily, "2026-10-02 04:00:00", "2026-10-02 16:30:00"},
		{daily, "2026-10-02 17:00:00", "2026-10-03 04:00:00"},
		{weekend, "2026-10-02 12:00:00", "2026-10-03 02:30:00"}, // Friday to Saturday
		{weekend, "2026-10-04 03:00:00", "2026-10-10 02:30:00"}, // Sunday to the next Saturday
		// 02:30 doesn't exist when the clocks go forward on 29 March 2026; it becomes 03:30.
		{weekend, "2026-03-28 03:00:00", "2026-03-29 03:30:00"},
	} {
		if got := tc.schedule.Next(at(tc.after)); !got.Equal(at(tc.want)) {
			t.Errorf("%v after %s = %s, want %s", tc.schedule, tc.after, got.In(berlin), tc.want)
		}
	}
	// The time zone of the schedule counts, not that of the time passed.
	if got := daily.Next(at("2026-10-02 12:00:00").UTC()); !got.Equal(at("2026-10-02 16:30:00")) {
		t.Errorf("next after a UTC time = %s", got)
	}
}

func TestNormalize(t *testing.T) {
	s := Schedule{Days: []time.Weekday{3, 1, 3}, Times: []string{"16:30", "04:00", "16:30"}, TimeZone: "UTC"}
	if msg := s.normalize(); msg != "" || !slices.Equal(s.Days, []time.Weekday{1, 3}) || !slices.Equal(s.Times, []string{"04:00", "16:30"}) {
		t.Fatalf("normalized %+v: %q", s, msg)
	}
	every := Schedule{Days: []time.Weekday{0, 1, 2, 3, 4, 5, 6}, Times: []string{"04:00"}, TimeZone: "UTC"}
	if every.normalize(); len(every.Days) != 0 {
		t.Fatalf("all weekdays = %v, want none, which means every day", every.Days)
	}
	for _, bad := range []Schedule{
		{Times: []string{"24:00"}, TimeZone: "UTC"},
		{Times: []string{"4:00"}, TimeZone: "UTC"},
		{Times: nil, TimeZone: "UTC"},
		{Times: []string{"04:00"}, TimeZone: "Mars/Olympus"},
		{Times: []string{"04:00"}},
		{Days: []time.Weekday{7}, Times: []string{"04:00"}, TimeZone: "UTC"},
	} {
		if bad.normalize() == "" {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestTargets(t *testing.T) {
	server := "aaaaaaaaaaaaaaaaaaaaaaaaaa"
	got, ok := targets([]Target{{"n1", server}, {"n1", ""}, {"n2", server}, {"n2", server}})
	if want := []Target{{"n1", ""}, {"n2", server}}; !ok || !slices.Equal(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	for _, bad := range [][]Target{nil, {{"", ""}}, {{"n1", "../x"}}} {
		if _, ok := targets(bad); ok {
			t.Errorf("accepted %v", bad)
		}
	}
}

// fakeKind records its runs.
type fakeKind struct{ runs chan time.Time }

func (fakeKind) Check(settings json.RawMessage) (json.RawMessage, error) { return settings, nil }
func (fakeKind) Lead(json.RawMessage) time.Duration                      { return 0 }
func (fakeKind) Category() slog.Attr                                     { return logging.System }
func (k fakeKind) Run(_ context.Context, _ Task, _ Servers, at time.Time) error {
	k.runs <- at
	return errors.New("node-1: unreachable")
}

type fakeNodes struct{}

func (fakeNodes) Get(context.Context, string) (node.Node, error) { return node.Node{}, nil }
func (fakeNodes) Conn(context.Context, string) (grpc.ClientConnInterface, error) {
	return nil, errors.New("offline")
}

func TestService(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "master.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO nodes (id, name, address, created_at) VALUES ('n1', 'node-1', 'x:1', 0)`); err != nil {
		t.Fatal(err)
	}
	kind := fakeKind{make(chan time.Time, 1)}
	s := NewService(db, fakeNodes{}, map[string]Kind{"fake": kind}, func(string) bool { return false })
	if err := s.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	in := Input{
		Name: "Nightly", Enabled: true, Settings: json.RawMessage(`{}`),
		Schedule: Schedule{Times: []string{"04:00"}, TimeZone: "UTC"}, Targets: []Target{{NodeID: "n1"}},
	}
	task, err := s.Create(t.Context(), "fake", in)
	if err != nil {
		t.Fatal(err)
	}
	if task.NextRun == nil || task.NextRun.UTC().Format("15:04") != "04:00" {
		t.Fatalf("next run = %v", task.NextRun)
	}
	if _, err := s.Create(t.Context(), "fake", in); err == nil {
		t.Fatal("created a second task with the same name")
	}
	if _, err := s.Create(t.Context(), "fake", Input{Name: "Other", Schedule: in.Schedule, Targets: []Target{{NodeID: "missing"}}}); err == nil {
		t.Fatal("created a task for a node that doesn't exist")
	}

	// A task that is due runs, and its outcome is recorded.
	s.mu.Lock()
	s.slots[task.ID].at = time.Now()
	s.mu.Unlock()
	s.plan(Task{}) // wakes the loop
	select {
	case <-kind.runs:
	case <-time.After(5 * time.Second):
		t.Fatal("the due task didn't run")
	}
	for task.LastRun == nil || task.Running {
		time.Sleep(10 * time.Millisecond)
		if task, err = s.Get(t.Context(), "fake", task.ID); err != nil {
			t.Fatal(err)
		}
	}
	if task.LastRun.Error != "node-1: unreachable" || task.NextRun == nil || time.Until(*task.NextRun) < time.Minute {
		t.Fatalf("after the run: %+v", task)
	}

	// Deleted servers are no longer targets, and disabled tasks aren't scheduled.
	in.Targets, in.Enabled = []Target{{NodeID: "n1", ServerID: "aaaaaaaaaaaaaaaaaaaaaaaaaa"}}, false
	if task, err = s.Update(t.Context(), "fake", task.ID, in); err != nil || task.NextRun != nil {
		t.Fatalf("disabled task: %+v, %v", task, err)
	}
	if err := s.Forget(t.Context(), "n1", "aaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if task, err = s.Get(t.Context(), "fake", task.ID); err != nil || len(task.Targets) != 0 {
		t.Fatalf("targets after forgetting the server: %+v, %v", task.Targets, err)
	}
	if _, err := s.Get(t.Context(), "other", task.ID); !errors.Is(err, errNotFound) {
		t.Fatalf("a task of another kind: %v", err)
	}
}
