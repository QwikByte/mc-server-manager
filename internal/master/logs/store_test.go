package logs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
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

// limits are what a test's log keeps.
type limits struct {
	retention time.Duration
	maxSize   int64
}

func (l *limits) LogRetention() time.Duration { return l.retention }
func (l *limits) LogMaxSize() int64           { return l.maxSize }

func newStore(t *testing.T) (*Store, context.Context) {
	db, err := database.Open(filepath.Join(t.TempDir(), "master.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db, NewNames(fakeNodes{}), &limits{24 * time.Hour, 1 << 30}), access.WithGrants(t.Context(), access.Admin())
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
	// More old entries than pruning deletes in a step.
	if _, err := s.db.ExecContext(ctx, `WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i <= ?)
		INSERT INTO log_entries (`+fields+`) SELECT ?, 0, 'master', 'system', 'old', '', '', '', '', '', '{}' FROM n`,
		pruneStep, time.Now().Add(-25*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if err := s.write(ctx, []Entry{{Time: time.Now(), Message: "new"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.prune(ctx); err != nil {
		t.Fatal(err)
	}
	if got := messages(t, s, ctx, Filter{}); got != "new" {
		t.Fatalf("after pruning: %q", got)
	}
}

// Beyond the size limit, the oldest entries are deleted; that they are younger than the
// retention is logged once, and again after entries lasted the whole retention.
func TestPruneBySize(t *testing.T) {
	s, ctx := newStore(t)
	s.conf = &limits{24 * time.Hour, 10_000}
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	written := 0
	add := func(n int, at time.Time) {
		t.Helper()
		batch := make([]Entry, n)
		for i := range batch {
			// 100 bytes of message, "system" and "{}" make 108 bytes of text, and 64 more.
			batch[i] = Entry{Time: at, Message: fmt.Sprintf("%03d%s", written, strings.Repeat("x", 97))}
			written++
		}
		if err := s.write(ctx, batch, nil); err != nil {
			t.Fatal(err)
		}
	}
	// prune checks the totals and returns the first entry left, their number and size.
	prune := func() (first string, entries, size int64) {
		t.Helper()
		if err := s.prune(ctx); err != nil {
			t.Fatal(err)
		}
		var counted, summed [2]int64
		if err := s.db.QueryRowContext(ctx, `SELECT entries, bytes FROM log_size`).Scan(&counted[0], &counted[1]); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(size), 0) FROM log_entries`).Scan(&summed[0], &summed[1]); err != nil {
			t.Fatal(err)
		}
		if counted != summed {
			t.Fatalf("log_size holds %v, the entries are %v", counted, summed)
		}
		oldest, err := s.List(ctx, Filter{}, true, 1)
		if err != nil || len(oldest) == 0 {
			t.Fatal(oldest, err)
		}
		return oldest[0].Message[:3], counted[0], counted[1]
	}
	warnings := func() int { return strings.Count(buf.String(), "The log reached its limit") }

	add(100, time.Now())
	if first, entries, size := prune(); first != "042" || entries != 58 || size != 58*172 || warnings() != 1 {
		t.Fatalf("after pruning 100 entries of 172 bytes to 10,000: %d entries of %d bytes from %s, %d warnings", entries, size, first, warnings())
	}
	if !strings.Contains(buf.String(), "limit=") || !strings.Contains(buf.String(), "deleted=42") {
		t.Errorf("warning without the limit or the deleted entries: %s", buf.String())
	}
	add(10, time.Now())
	if first, _, _ := prune(); first != "052" || warnings() != 1 {
		t.Fatalf("a log that stays full: first entry %s, %d warnings", first, warnings())
	}
	add(1, time.Now().Add(-25*time.Hour))
	if _, entries, _ := prune(); entries != 58 || warnings() != 1 {
		t.Fatalf("after deleting an expired entry: %d entries, %d warnings", entries, warnings())
	}
	add(10, time.Now())
	if first, _, _ := prune(); first != "062" || warnings() != 2 {
		t.Fatalf("full again after entries lasted the retention: first entry %s, %d warnings", first, warnings())
	}

	s.conf = &limits{24 * time.Hour, 1 << 30}
	add(10, time.Now())
	if _, entries, _ := prune(); entries != 68 || warnings() != 2 {
		t.Fatalf("below the limit: %d entries, %d warnings", entries, warnings())
	}
}

func TestSpreadsheetSafe(t *testing.T) {
	for in, want := range map[string]string{"=HYPERLINK(1)": "'=HYPERLINK(1)", "-1": "'-1", "@x": "'@x", "Start server": "Start server", "": ""} {
		if got := spreadsheetSafe(in); got != want {
			t.Errorf("spreadsheetSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAgentBudget(t *testing.T) {
	big := map[string]string{}
	for i := range maxAttrs {
		big[strings.Repeat("k", maxName)+string(rune('a'+i))] = strings.Repeat("<", maxValue)
	}
	e := Entry{Message: strings.Repeat("m", 3*maxMessage), Attrs: big}
	e.clamp()
	size := len(e.Message)
	for k, v := range e.Attrs {
		size += len(k) + len(v)
	}
	if size > maxMessage+maxAttrsSize+20 {
		t.Errorf("clamped entry has %d bytes", size)
	}

	limits := budget{rate.NewLimiter(agentRate, agentBurst), rate.NewLimiter(agentBytes, agentBytesBurst)}
	now, allowed := time.Now(), 0
	for range agentBurst {
		e := Entry{Message: "m", Attrs: big}
		if limits.allow(now, &e) {
			allowed++
		}
	}
	// Each entry encodes "<" as six bytes, so the bytes run out long before the entries.
	if allowed == 0 || allowed >= agentBurst/10 {
		t.Errorf("%d of %d large entries allowed at once", allowed, agentBurst)
	}
}
