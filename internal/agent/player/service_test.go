package player

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// fakeRuntime has servers with data directories and records console commands.
type fakeRuntime struct {
	runtime.Runtime
	dir      string
	mu       sync.Mutex
	servers  []runtime.Server
	commands []string
}

func (f *fakeRuntime) List(context.Context) ([]runtime.Server, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.servers), nil
}

func (f *fakeRuntime) Data(_ context.Context, id string) (*datadir.Dir, error) {
	return datadir.Open(filepath.Join(f.dir, id))
}

func (f *fakeRuntime) SendCommand(_ context.Context, id, command string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, id[:1]+" "+command)
	return "§aBanned Alex: Cheating\n", nil
}

func (f *fakeRuntime) add(t *testing.T, id string, typ noryxv1.ServerType, state noryxv1.ServerState) {
	t.Helper()
	if err := os.Mkdir(filepath.Join(f.dir, id), 0o750); err != nil {
		t.Fatal(err)
	}
	f.servers = append(f.servers, runtime.Server{Spec: runtime.Spec{ID: id, Type: typ}, State: state})
}

func (f *fakeRuntime) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.commands)
}

var (
	paper   = noryxv1.ServerType_SERVER_TYPE_PAPER
	stopped = noryxv1.ServerState_SERVER_STATE_STOPPED
	// IDs of a running server, a stopped one and a proxy.
	up, down, proxy = "aaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccc"
)

func newChange(action noryxv1.PlayerAction, name, reason string) *noryxv1.PlayerChange {
	return &noryxv1.PlayerChange{Action: action, Name: name, Reason: reason}
}

func TestChangePlayer(t *testing.T) {
	ctx := t.Context()
	rt := &fakeRuntime{dir: t.TempDir()}
	rt.add(t, up, paper, running)
	rt.add(t, down, paper, stopped)
	rt.add(t, proxy, noryxv1.ServerType_SERVER_TYPE_VELOCITY, running)
	s := NewService(rt)
	ban := newChange(noryxv1.PlayerAction_PLAYER_ACTION_BAN, "Alex", "Cheating")

	// A running server runs the command at once.
	res, err := s.ChangePlayer(ctx, &noryxv1.ChangePlayerRequest{ServerId: up, Change: ban})
	if err != nil || res.GetPending() || res.GetOutput() != "Banned Alex: Cheating" {
		t.Fatalf("ban on a running server = %v, %v", res, err)
	}

	// Invalid changes, proxies and kicks from stopped servers are refused.
	for _, tc := range []struct {
		server string
		change *noryxv1.PlayerChange
		want   codes.Code
	}{
		{up, newChange(noryxv1.PlayerAction_PLAYER_ACTION_BAN, "@a", ""), codes.InvalidArgument},
		{up, newChange(noryxv1.PlayerAction_PLAYER_ACTION_OP, "Alex extra", ""), codes.InvalidArgument},
		{up, newChange(noryxv1.PlayerAction_PLAYER_ACTION_PARDON, "Alex", "why"), codes.InvalidArgument},
		{up, newChange(noryxv1.PlayerAction_PLAYER_ACTION_KICK, "Alex", "a\nsay hi"), codes.InvalidArgument},
		{up, newChange(noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_ON, "Alex", ""), codes.InvalidArgument},
		{up, newChange(noryxv1.PlayerAction_PLAYER_ACTION_UNSPECIFIED, "Alex", ""), codes.InvalidArgument},
		{proxy, ban, codes.FailedPrecondition},
		{down, newChange(noryxv1.PlayerAction_PLAYER_ACTION_KICK, "Alex", ""), codes.FailedPrecondition},
	} {
		if _, err := s.ChangePlayer(ctx, &noryxv1.ChangePlayerRequest{ServerId: tc.server, Change: tc.change}); status.Code(err) != tc.want {
			t.Errorf("%v on %s: %v, want %v", tc.change, tc.server[:1], err, tc.want)
		}
	}

	// A stopped server gets the changes once it runs, each once and in order.
	for _, c := range []*noryxv1.PlayerChange{ban, ban, newChange(noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_ON, "", "")} {
		if res, err := s.ChangePlayer(ctx, &noryxv1.ChangePlayerRequest{ServerId: down, Change: c}); err != nil || !res.GetPending() {
			t.Fatalf("change on a stopped server = %v, %v", res, err)
		}
	}
	lists, err := s.GetPlayerLists(ctx, &noryxv1.GetPlayerListsRequest{ServerId: down})
	if err != nil || len(lists.GetPending()) != 2 {
		t.Fatalf("waiting changes = %v, %v", lists, err)
	}
	s.applyWaiting(ctx)
	if got := rt.sent(); len(got) != 1 {
		t.Fatalf("commands before the server runs: %q", got)
	}
	rt.servers[1].State = running
	s.applyWaiting(ctx)
	want := []string{"a minecraft:ban Alex Cheating", "b minecraft:ban Alex Cheating", "b minecraft:whitelist on"}
	if got := rt.sent(); !slices.Equal(got, want) {
		t.Fatalf("commands = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(rt.dir, down, pendingFile)); !os.IsNotExist(err) {
		t.Fatalf("the waiting changes stay: %v", err)
	}
	s.applyWaiting(ctx)
	if got := rt.sent(); len(got) != 3 {
		t.Fatalf("changes ran again: %q", got)
	}
}

func TestGetPlayerLists(t *testing.T) {
	rt := &fakeRuntime{dir: t.TempDir()}
	rt.add(t, up, paper, running)
	files := map[string]string{
		bannedFile: `[{"uuid":"u1","name":"Alex","created":"2026-05-01 10:00:00 +0200","source":"Server","expires":"forever","reason":"Cheating"},
			{"uuid":"u3","name":"Kai","created":"2026-05-01 10:00:00 +0200","source":"Essentials","expires":"2026-05-08 10:00:00 +0200","reason":"Spam"},
			{"uuid":"u4","name":"Bo","created":"2026-05-01 10:00:00 +0200","source":"Plugin","expires":"next week","reason":"Grief"},
			{"uuid":"u5","name":"Lu","expires":"2026-05-08T10:00:00Z"}]`,
		whitelistFile:       `[{"uuid":"u2","name":"Steve"},{"uuid":"u1","name":"Alex"}]`,
		opsFile:             `[{"uuid":"u2","name":"Steve","level":4,"bypassesPlayerLimit":false}]`,
		"server.properties": "white-list=true\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(rt.dir, up, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lists, err := NewService(rt).GetPlayerLists(t.Context(), &noryxv1.GetPlayerListsRequest{ServerId: up})
	if err != nil {
		t.Fatal(err)
	}
	ban := lists.GetBanned()[0]
	if len(lists.GetBanned()) != 4 || ban.GetName() != "Alex" || ban.GetReason() != "Cheating" || ban.GetCreatedUnix() != 1777622400 {
		t.Errorf("banned = %v", lists.GetBanned())
	}
	// A temporary ban ends at its time; other values than Minecraft's times count as forever.
	for i, want := range []int64{0, 1778227200, 0, 0} {
		if got := lists.GetBanned()[i].GetExpiresUnix(); got != want {
			t.Errorf("ban of %s expires at %d, want %d", lists.GetBanned()[i].GetName(), got, want)
		}
	}
	if len(lists.GetWhitelisted()) != 2 || lists.GetOperators()[0].GetName() != "Steve" || !lists.GetWhitelistEnabled() {
		t.Errorf("lists = %v", lists)
	}
}
