package schedule

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/tag"
)

// Tasks that need permissions beyond the one to manage them need them from whoever saves them
// or runs them by hand, and their runs from the user who saved them last.
func TestAuthors(t *testing.T) {
	db := openDB(t)
	if _, err := db.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'admin', '', 0)`); err != nil {
		t.Fatal(err)
	}
	users := access.NewService(db)
	if err := users.MakeAdmin(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	kind := fakeKind{make(chan []string, 1)}
	s := NewService(db, &fakeNodes{servers: map[string][]string{"n1": {serverA}}}, tag.NewStore(db), fakeNetworks{}, users, map[string]Kind{"fake": kind}, func(string) bool { return false })
	admin, nobody := Author{ID: 1, Name: "admin", Grants: access.Admin()}, Author{Name: "nobody"}
	in := Input{
		Name: "Backed up", Schedule: Schedule{Times: []string{"04:00"}, TimeZone: "UTC"}, Targets: []Target{{NodeID: "n1"}},
		Settings: json.RawMessage(`{"needs":["backups.create"]}`),
	}
	if _, err := s.Create(t.Context(), "fake", in, nobody); err == nil {
		t.Fatal("saved without the permission the task needs")
	}
	task, err := s.Create(t.Context(), "fake", in, admin)
	if err != nil || task.SavedBy != "admin" {
		t.Fatalf("created %+v, %v", task, err)
	}
	if _, err := s.RunNow(t.Context(), "fake", task.ID, nobody); err == nil {
		t.Fatal("ran without the permission the task needs")
	}
	// run runs the task, which fails with want unless it is empty, and returns the run.
	run := func(want string) Run {
		t.Helper()
		if _, err := s.RunNow(t.Context(), "fake", task.ID, admin); err != nil {
			t.Fatal(err)
		}
		if want == "" {
			<-kind.runs
		}
		for {
			got, err := s.Get(t.Context(), "fake", task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Running && got.LastRun != nil && (task.LastRun == nil || got.LastRun.ID != task.LastRun.ID) {
				task = got
				if !strings.Contains(got.LastRun.Error, want) || want == "" && got.LastRun.Error != "" {
					t.Fatalf("error = %q, want %q", got.LastRun.Error, want)
				}
				return *got.LastRun
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if r := run(""); r.Outcome != Succeeded || len(r.Steps) != 1 || r.Steps[0] != (Step{"Touch server", serverA, "node-n1", Succeeded, "", "touched"}) {
		t.Fatalf("run = %+v", r)
	}
	// The user who saved it loses the permission, is disabled or deleted.
	if _, err := db.Exec(`DELETE FROM group_members`); err != nil {
		t.Fatal(err)
	}
	run(`admin, who saved it last, no longer has the permission "Back up servers"`)
	if _, err := db.Exec(`UPDATE users SET disabled = 1`); err != nil {
		t.Fatal(err)
	}
	run("the user who saved it last was deleted or disabled")
	if _, err := db.Exec(`DELETE FROM users`); err != nil {
		t.Fatal(err)
	}
	if r := run("the user who saved it last was deleted or disabled"); r.Outcome != Failed || task.SavedBy != "" {
		t.Fatalf("run = %+v, saved by %q", r, task.SavedBy)
	}

	// A run whose steps all left their servers out is skipped.
	in.Settings = json.RawMessage(`{"skip":true}`)
	if task, err = s.Update(t.Context(), "fake", task.ID, in, nobody); err != nil {
		t.Fatal(err)
	}
	if r := run(""); r.Outcome != OutcomeSkipped || r.Note != serverA+" on node-n1: Players were online." || r.Steps[0].Outcome != OutcomeSkipped {
		t.Fatalf("run = %+v", r)
	}
}

// A run stops waiting once its task is paused or deleted, but not when it is saved otherwise.
func TestWithdrawn(t *testing.T) {
	db := openDB(t)
	kind := fakeKind{make(chan []string, 1)}
	s := NewService(db, &fakeNodes{servers: map[string][]string{"n1": {serverA}}}, tag.NewStore(db), fakeNetworks{}, access.NewService(db), map[string]Kind{"fake": kind}, func(string) bool { return false })
	in := Input{
		Name: "Waits", Enabled: true, Schedule: Schedule{Times: []string{"04:00"}, TimeZone: "UTC"}, Targets: []Target{{NodeID: "n1"}},
		Settings: json.RawMessage(`{"wait":true}`),
	}
	task, err := s.Create(t.Context(), "fake", in, Author{})
	if err != nil {
		t.Fatal(err)
	}
	for _, withdraw := range []func() error{
		func() error {
			paused := in
			paused.Enabled = false
			_, err := s.Update(t.Context(), "fake", task.ID, paused, Author{})
			return err
		},
		func() error { return s.Delete(t.Context(), "fake", task.ID) },
	} {
		if _, err := s.Update(t.Context(), "fake", task.ID, in, Author{}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RunNow(t.Context(), "fake", task.ID, Author{}); err != nil {
			t.Fatal(err)
		}
		<-kind.runs
		if _, err := s.Update(t.Context(), "fake", task.ID, in, Author{}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
		if len(s.runningTasks()) != 1 {
			t.Fatal("the run stopped waiting when its task was saved")
		}
		if err := withdraw(); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(5 * time.Second); len(s.runningTasks()) > 0; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("the run kept waiting")
			}
		}
	}
	if _, err := s.Get(t.Context(), "fake", task.ID); !errors.Is(err, errNotFound) {
		t.Fatalf("deleted task: %v", err)
	}
}
