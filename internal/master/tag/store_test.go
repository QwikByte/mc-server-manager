package tag

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/QwikByte/noryx/internal/master/database"
)

func TestNormalize(t *testing.T) {
	got, err := Normalize([]string{" Lobby ", "bed_wars", "lobby", "Über-1"})
	if want := []string{"bed_wars", "lobby", "über-1"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("Normalize = %q, %v, want %q", got, err, want)
	}
	for _, bad := range []string{"", "two words", "<b>", "a-very-long-tag-for-a-server"} {
		if _, err := Normalize([]string{bad}); err == nil {
			t.Errorf("Normalize accepted %q", bad)
		}
	}
}

func TestStore(t *testing.T) {
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
	s, ctx := NewStore(db), t.Context()
	lobby, game := Server{"n1", "lobby"}, Server{"n1", "game"}
	check := func(want map[Server][]string) {
		t.Helper()
		got, err := s.All(ctx)
		if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("All = %v, %v, want %v", got, err, want)
		}
	}

	if err := s.Change(ctx, []Server{lobby, game}, []string{"EU", "lobby"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.Change(ctx, []Server{game}, []string{"bedwars"}, []string{"lobby", "unknown"}); err != nil {
		t.Fatal(err)
	}
	check(map[Server][]string{lobby: {"eu", "lobby"}, game: {"bedwars", "eu"}})

	// A change that gives a server too many tags changes nothing.
	many := make([]string, MaxPerServer-1)
	for i := range many {
		many[i] = fmt.Sprint("t", i)
	}
	if err := s.Change(ctx, []Server{lobby, game}, many, []string{"lobby"}); err == nil {
		t.Fatal("too many tags were accepted")
	}
	check(map[Server][]string{lobby: {"eu", "lobby"}, game: {"bedwars", "eu"}})

	// Tags follow copies and moves, and go with the server.
	copied, moved := Server{"n1", "copy"}, Server{"n2", "lobby"}
	if err := s.Copy(ctx, game, copied); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(ctx, "lobby", "n1", "n2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget(ctx, "n1", "game"); err != nil {
		t.Fatal(err)
	}
	check(map[Server][]string{copied: {"bedwars", "eu"}, moved: {"eu", "lobby"}})
}
