package logs

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/node"
)

type fakeNodes struct{}

func (fakeNodes) List(context.Context) ([]node.Node, error) { return nil, nil }
func (fakeNodes) Get(_ context.Context, id string) (node.Node, error) {
	if id == "n1" {
		return node.Node{ID: id, Name: "node-1"}, nil
	}
	return node.Node{}, errors.New("not found")
}
func (fakeNodes) Conn(context.Context, string) (grpc.ClientConnInterface, error) {
	return nil, errors.New("offline")
}

func newStore(t *testing.T) (*Store, context.Context) {
	db, err := database.Open(filepath.Join(t.TempDir(), "master.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db, NewNames(fakeNodes{}), func() time.Duration { return 24 * time.Hour }), access.WithGrants(t.Context(), access.Admin())
}

func messages(t *testing.T, s *Store, ctx context.Context, f Filter) string {
	t.Helper()
	entries, err := s.List(ctx, f, true, 100)
	if err != nil {
		t.Fatal(err)
	}
	var m []string
	for _, e := range entries {
		m = append(m, e.Message)
	}
	return strings.Join(m, ",")
}

func TestStore(t *testing.T) {
	s, ctx := newStore(t)
	s.Start(ctx)
	log := slog.New(s.Handler(slog.LevelInfo))
	log.Debug("hidden")
	log.Info("a", logging.Servers, logging.KeyUser, "alice", logging.KeyNode, "n1", logging.KeyServer, "s1")
	log.Warn("b", logging.Category("Not a category"), "note", "disk 50% full")
	log.Error("c", logging.Auth, "note", "disk 500 full")
	long := strings.Repeat("ü", maxMessage)
	attrs := make([]any, 0, 2*(maxAttrs+5))
	for i := range maxAttrs + 5 {
		attrs = append(attrs, string(rune('A'+i)), i)
	}
	log.Info(long, attrs...)
	s.Close()

	entries, err := s.List(ctx, Filter{}, true, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}
	if a := entries[0]; a.Category != "servers" || a.User != "alice" || a.NodeName != "node-1" || a.ServerID != "s1" || a.Source != FromMaster {
		t.Errorf("entry a = %+v", a)
	}
	if b := entries[1]; b.Category != "system" || b.Level != slog.LevelWarn {
		t.Errorf("an invalid category is kept: %+v", b)
	}
	if d := entries[3]; len(d.Message) > maxMessage+len("…") || !strings.HasSuffix(d.Message, "ü…") || len(d.Attrs) != maxAttrs {
		t.Errorf("a large entry is kept: %d bytes, %d attributes", len(d.Message), len(d.Attrs))
	}

	for _, tc := range []struct {
		name string
		f    Filter
		want string
	}{
		{"level", Filter{Level: "warn"}, "b,c"},
		{"category", Filter{Category: "auth"}, "c"},
		{"user", Filter{User: "alice"}, "a"},
		{"server", Filter{NodeID: "n1", ServerID: "s1"}, "a"},
		{"search with a wildcard", Filter{Search: "50%"}, "b"},
		{"search for names", Filter{Search: "NODE-1"}, "a"},
		{"after", Filter{After: entries[1].ID}, "c," + entries[3].Message},
		{"before", Filter{Before: entries[1].ID}, "a"},
		{"time", Filter{Until: entries[0].Time.Add(-time.Second)}, ""},
	} {
		if got := messages(t, s, ctx, tc.f); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := messages(t, s, t.Context(), Filter{}); got != "" {
		t.Errorf("without permissions: got %q", got)
	}

	buckets, err := s.Stats(ctx, Filter{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if now := buckets[2]; len(buckets) != 3 || now.Info != 2 || now.Warn != 1 || now.Error != 1 {
		t.Errorf("stats = %+v", buckets)
	}
}

func TestPrune(t *testing.T) {
	s, ctx := newStore(t)
	old := Entry{Time: time.Now().Add(-25 * time.Hour), Message: "old"}
	if err := s.write(ctx, []Entry{old, {Time: time.Now(), Message: "new"}}, nil); err != nil {
		t.Fatal(err)
	}
	s.prune(ctx)
	if got := messages(t, s, ctx, Filter{}); got != "new" {
		t.Fatalf("after pruning: %q", got)
	}
}

func TestSpreadsheetSafe(t *testing.T) {
	for in, want := range map[string]string{"=HYPERLINK(1)": "'=HYPERLINK(1)", "-1": "'-1", "@x": "'@x", "Start server": "Start server", "": ""} {
		if got := spreadsheetSafe(in); got != want {
			t.Errorf("spreadsheetSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
