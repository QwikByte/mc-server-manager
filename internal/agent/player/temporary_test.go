package player

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// A temporary ban goes out at once, and the pardon at its end waits, also for a server that
// doesn't run then or an agent that restarted. A later ban or pardon replaces it.
func TestTemporaryBans(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		rt := &fakeRuntime{dir: t.TempDir()}
		rt.add(t, up, paper, running)
		rt.add(t, down, paper, stopped)
		s := NewService(rt)
		ends := time.Now().Add(time.Hour).Unix()
		temporary := func(name string) *noryxv1.PlayerChange {
			return &noryxv1.PlayerChange{Action: noryxv1.PlayerAction_PLAYER_ACTION_BAN, Name: name, Reason: "Spam", EndsUnix: ends}
		}
		changes := func(id string, cs ...*noryxv1.PlayerChange) []*noryxv1.PlayerChangeResult {
			t.Helper()
			res, err := s.ChangePlayers(ctx, &noryxv1.ChangePlayersRequest{ServerId: id, Changes: cs})
			if err != nil || len(res.GetResults()) != len(cs) {
				t.Fatalf("changes on %s = %v, %v", id[:1], res, err)
			}
			return res.GetResults()
		}
		pending := func(id string) []string {
			t.Helper()
			lists, err := s.GetPlayerLists(ctx, &noryxv1.GetPlayerListsRequest{ServerId: id})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, p := range lists.GetPending() {
				got = append(got, strings.TrimSpace(p.GetAction().Slug()+" "+p.GetName()+" "+map[bool]string{true: "later"}[p.GetDueUnix() > 0]))
			}
			return got
		}
		run := func() {
			s.applyWaiting(ctx)
			synctest.Wait()
		}

		// Several players are banned at once on the running server, and wait for the stopped one.
		if r := changes(up, temporary("Alex"), temporary("Steve")); r[0].GetPending() || r[0].GetError() != "" || r[1].GetOutput() != "Banned Alex: Cheating" {
			t.Fatalf("results = %v", r)
		}
		if r := changes(down, temporary("Alex")); !r[0].GetPending() {
			t.Fatalf("results = %v", r)
		}
		if got := pending(up); !slices.Equal(got, []string{"pardon Alex later", "pardon Steve later"}) {
			t.Fatalf("waiting on the running server = %q", got)
		}
		if got := pending(down); !slices.Equal(got, []string{"ban Alex", "pardon Alex later"}) {
			t.Fatalf("waiting on the stopped server = %q", got)
		}
		// The lists tell when the ban ends.
		if err := os.WriteFile(filepath.Join(rt.dir, up, bannedFile), []byte(`[{"name":"Alex","expires":"forever"},{"name":"Kai","expires":"forever"}]`), 0o600); err != nil {
			t.Fatal(err)
		}
		lists, _ := s.GetPlayerLists(ctx, &noryxv1.GetPlayerListsRequest{ServerId: up})
		if b := lists.GetBanned(); b[0].GetExpiresUnix() != ends || b[1].GetExpiresUnix() != 0 {
			t.Fatalf("banned = %v", b)
		}

		// Nothing is pardoned before its time, and a pardon by hand replaces the waiting one.
		run()
		changes(up, &noryxv1.PlayerChange{Action: noryxv1.PlayerAction_PLAYER_ACTION_PARDON, Name: "steve"})
		if got := pending(up); !slices.Equal(got, []string{"pardon Alex later"}) {
			t.Fatalf("waiting after a pardon = %q", got)
		}
		want := []string{"a minecraft:ban Alex Spam", "a minecraft:ban Steve Spam", "a minecraft:pardon steve"}
		if got := rt.sent(); !slices.Equal(got, want) {
			t.Fatalf("commands = %q, want %q", got, want)
		}

		// At its time the pardon runs; the stopped server keeps its changes until it runs.
		time.Sleep(time.Hour)
		run()
		if got := rt.sent()[3:]; !slices.Equal(got, []string{"a minecraft:pardon Alex"}) || len(pending(up)) != 0 || len(pending(down)) != 2 {
			t.Fatalf("commands = %q", got)
		}
		rt.servers[1].State = running
		run()
		if got := rt.sent()[4:]; !slices.Equal(got, []string{"b minecraft:ban Alex Spam", "b minecraft:pardon Alex"}) || len(pending(down)) != 0 {
			t.Fatalf("commands = %q", got)
		}

		// A ban for good replaces the pardon of a temporary one.
		ends = time.Now().Add(time.Hour).Unix()
		changes(up, temporary("Kai"), temporary("Bo"), &noryxv1.PlayerChange{Action: noryxv1.PlayerAction_PLAYER_ACTION_BAN, Name: "Kai"})
		if got := pending(up); !slices.Equal(got, []string{"pardon Bo later"}) {
			t.Fatalf("waiting after a ban for good = %q", got)
		}

		// An agent that restarted pardons at the time too.
		s = NewService(rt)
		time.Sleep(time.Hour)
		run()
		if got := rt.sent(); got[len(got)-1] != "a minecraft:pardon Bo" || len(pending(up)) != 0 {
			t.Fatalf("commands = %q", got)
		}

		// Requests can't make changes wait for a time, and only bans end.
		for _, c := range []*noryxv1.PlayerChange{
			{Action: noryxv1.PlayerAction_PLAYER_ACTION_PARDON, Name: "Alex", DueUnix: ends},
			{Action: noryxv1.PlayerAction_PLAYER_ACTION_PARDON, Name: "Alex", EndsUnix: ends},
			{Action: noryxv1.PlayerAction_PLAYER_ACTION_BAN, Name: "Alex", EndsUnix: -1},
		} {
			if _, err := s.ChangePlayers(ctx, &noryxv1.ChangePlayersRequest{ServerId: up, Changes: []*noryxv1.PlayerChange{c}}); status.Code(err) != codes.InvalidArgument {
				t.Errorf("%v: %v", c, err)
			}
		}
		many := slices.Repeat([]*noryxv1.PlayerChange{temporary("Alex")}, noryxv1.MaxPlayerChanges+1)
		if _, err := s.ChangePlayers(ctx, &noryxv1.ChangePlayersRequest{ServerId: up, Changes: many}); status.Code(err) != codes.InvalidArgument {
			t.Errorf("too many changes: %v", err)
		}
	})
}

// The players who joined come from the server's cache, with valid names only.
func TestJoined(t *testing.T) {
	rt := &fakeRuntime{dir: t.TempDir()}
	rt.add(t, up, paper, running)
	cache := `[{"name":"Alex","uuid":"u1","expiresOn":"2026-11-01 10:00:00 +0000"},{"name":"@a","uuid":"u2"},{"name":".Bedrock_Kid","uuid":"u3"}]`
	if err := os.WriteFile(filepath.Join(rt.dir, up, cacheFile), []byte(cache), 0o600); err != nil {
		t.Fatal(err)
	}
	lists, err := NewService(rt).GetPlayerLists(t.Context(), &noryxv1.GetPlayerListsRequest{ServerId: up})
	if err != nil {
		t.Fatal(err)
	}
	if j := lists.GetJoined(); len(j) != 2 || j[0].GetName() != "Alex" || j[0].GetUuid() != "u1" || j[1].GetName() != ".Bedrock_Kid" {
		t.Fatalf("joined = %v", j)
	}
	// A broken cache leaves out only the players who joined.
	if err := os.WriteFile(filepath.Join(rt.dir, up, cacheFile), []byte(`{broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	if lists, err := NewService(rt).GetPlayerLists(t.Context(), &noryxv1.GetPlayerListsRequest{ServerId: up}); err != nil || len(lists.GetJoined()) != 0 {
		t.Fatalf("lists with a broken cache = %v, %v", lists, err)
	}
}
