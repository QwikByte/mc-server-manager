package usage

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/database"
)

const (
	lobby   = "lobbylobbylobbylobbylobbya"
	stopped = "stoppedstoppedstoppedstopp"
)

func TestHistory(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{"n1", "n2"} {
		if _, err := db.Exec(`INSERT INTO nodes (id, name, address, created_at) VALUES (?, ?, 'host:7443', 0)`, id, id); err != nil {
			t.Fatal(err)
		}
	}
	s, ctx := NewStore(db, nil), t.Context()
	step := 5 * time.Minute
	start := time.Now().Truncate(step).Add(-step)
	for i, players := range []uint32{3, 7} {
		stats := &noryxv1.GetStatsResponse{
			Node: &noryxv1.NodeStats{CpuMillis: 2000, MemoryUsedBytes: 4 << 30},
			Servers: []*noryxv1.ServerStats{
				{Id: lobby, Running: true, CpuMillis: uint32(500 * (i + 1)), MemoryBytes: 1 << 30, DiskBytes: 100, Tps: 19.5, Players: &noryxv1.Players{Online: players}}, //nolint:gosec // small
				{Id: stopped, DiskBytes: 100},
			},
		}
		if err := s.add(ctx, "n1", start.Add(time.Duration(i)*time.Minute), stats); err != nil {
			t.Fatal(err)
		}
	}

	// Samples of a step are averaged, players are the most of the step.
	points, err := s.History(ctx, "n1", lobby, time.Hour, step)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].CPUMillis != 750 || *points[0].Players != 7 || *points[0].TPS != 19.5 || !points[0].Time.Equal(start) {
		t.Fatalf("points = %+v", points)
	}
	if points, _ := s.History(ctx, "n1", stopped, time.Hour, step); len(points) != 0 {
		t.Fatalf("stopped server recorded: %+v", points)
	}
	if points, _ := s.History(ctx, "n1", "", time.Hour, step); len(points) != 1 || points[0].Players != nil || points[0].MemoryBytes != 4<<30 {
		t.Fatalf("node = %+v", points)
	}

	// The history moves with the server and goes away with it.
	if err := s.Move(ctx, lobby, "n1", "n2"); err != nil {
		t.Fatal(err)
	}
	if points, _ := s.History(ctx, "n2", lobby, time.Hour, step); len(points) != 1 {
		t.Fatalf("moved history = %+v", points)
	}
	if err := s.Forget(ctx, "n2", lobby); err != nil {
		t.Fatal(err)
	}
	if points, _ := s.History(ctx, "n2", lobby, time.Hour, step); len(points) != 0 {
		t.Fatalf("forgotten history = %+v", points)
	}
}

func TestAddLimits(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO nodes (id, name, address, created_at) VALUES ('n1', 'n1', 'host:7443', 0)`); err != nil {
		t.Fatal(err)
	}
	// A compromised agent reports invalid, repeated and far too many servers.
	stats := &noryxv1.GetStatsResponse{Node: &noryxv1.NodeStats{}}
	for _, id := range []string{"../x", strings.Repeat("a", 10000), lobby, lobby} {
		stats.Servers = append(stats.Servers, &noryxv1.ServerStats{Id: id, Running: true})
	}
	for i := range 2 * maxServers {
		id := strings.Map(func(r rune) rune { return 'a' + r - '0' }, fmt.Sprintf("%026d", i)) // digits as letters
		stats.Servers = append(stats.Servers, &noryxv1.ServerStats{Id: id, Running: true})
	}
	if err := NewStore(db, nil).add(t.Context(), "n1", time.Now(), stats); err != nil {
		t.Fatal(err)
	}
	var servers, lobbies int
	if err := db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE server_id = ?) FROM usage_samples WHERE server_id != ''`, lobby).Scan(&servers, &lobbies); err != nil {
		t.Fatal(err)
	}
	if servers != maxServers || lobbies != 1 {
		t.Errorf("recorded %d servers, the lobby %d times", servers, lobbies)
	}
}
