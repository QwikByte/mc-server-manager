// Package player manages the players of game servers with the commands and lists of
// Minecraft: kicks, bans, the whitelist and operators. A server that doesn't run gets a
// change once it runs, so that a ban also reaches the stopped servers of a network.
package player

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/properties"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

const (
	running  = noryxv1.ServerState_SERVER_STATE_RUNNING
	interval = 5 * time.Second
	// changeTimeout bounds each waiting change, so that a server that doesn't answer its
	// console keeps the change for the next attempt.
	changeTimeout = 15 * time.Second
)

// commands are the console commands of the actions, which take the player and the reason.
var commands = map[noryxv1.PlayerAction]string{
	noryxv1.PlayerAction_PLAYER_ACTION_KICK:             "kick",
	noryxv1.PlayerAction_PLAYER_ACTION_BAN:              "ban",
	noryxv1.PlayerAction_PLAYER_ACTION_PARDON:           "pardon",
	noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_ADD:    "whitelist add",
	noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_REMOVE: "whitelist remove",
	noryxv1.PlayerAction_PLAYER_ACTION_OP:               "op",
	noryxv1.PlayerAction_PLAYER_ACTION_DEOP:             "deop",
	noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_ON:     "whitelist on",
	noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_OFF:    "whitelist off",
}

var formatting = regexp.MustCompile(`§.`)

type Service struct {
	noryxv1.UnimplementedPlayerServiceServer
	rt    runtime.Runtime
	locks serverLocks    // guard the file of waiting changes and the whitelist of each server
	wg    sync.WaitGroup // the attempts at the waiting changes of servers

	mu sync.Mutex // guards waiting and applying; never held while a server is asked
	// waiting tells which servers have changes that wait for them to run; servers the
	// agent didn't look at yet are missing.
	waiting map[string]bool
	// applying are the servers whose waiting changes are being attempted.
	applying map[string]bool
}

func NewService(rt runtime.Runtime) *Service {
	return &Service{rt: rt, locks: serverLocks{byID: map[string]*serverLock{}}, waiting: map[string]bool{}, applying: map[string]bool{}}
}

func (s *Service) GetPlayerLists(ctx context.Context, req *noryxv1.GetPlayerListsRequest) (*noryxv1.GetPlayerListsResponse, error) {
	_, dir, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	res := &noryxv1.GetPlayerListsResponse{}
	var errs [5]error
	res.Banned, errs[0] = readList(dir, bannedFile)
	res.Whitelisted, errs[1] = readList(dir, whitelistFile)
	res.Operators, errs[2] = readList(dir, opsFile)
	var props map[string]string
	props, errs[3] = properties.Read(dir)
	res.WhitelistEnabled = props["white-list"] == "true"
	res.Pending, errs[4] = readPending(dir) // its file is replaced at once, so it needs no lock
	if err := errors.Join(errs[:]...); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *Service) ChangePlayer(ctx context.Context, req *noryxv1.ChangePlayerRequest) (*noryxv1.ChangePlayerResponse, error) {
	change := req.GetChange()
	if msg := change.Problem(); msg != "" {
		return nil, status.Error(codes.InvalidArgument, msg)
	}
	srv, dir, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if srv.State == running {
		return s.run(ctx, srv.ID, dir, change)
	}
	if change.GetAction() == noryxv1.PlayerAction_PLAYER_ACTION_KICK {
		return nil, status.Error(codes.FailedPrecondition, "The server doesn't run.")
	}
	unlock, err := s.locks.lock(ctx, srv.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := addPending(dir, change); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.waiting[srv.ID] = true
	s.mu.Unlock()
	return &noryxv1.ChangePlayerResponse{Pending: true}, nil
}

// run changes a player on a running server. Changes of the whitelist hold the server's lock,
// as those of Bedrock players replace its file, which the server overwrites when its list
// changes.
func (s *Service) run(ctx context.Context, id string, dir *datadir.Dir, change *noryxv1.PlayerChange) (*noryxv1.ChangePlayerResponse, error) {
	if a := change.GetAction(); a == noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_ADD || a == noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_REMOVE {
		unlock, err := s.locks.lock(ctx, id)
		if err != nil {
			return nil, err
		}
		defer unlock()
	}
	if bedrockWhitelist(change) {
		return s.whitelistBedrock(ctx, id, dir, change)
	}
	out, err := s.rt.SendCommand(ctx, id, command(change))
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	return &noryxv1.ChangePlayerResponse{Output: strings.TrimSpace(formatting.ReplaceAllString(out, ""))}, nil
}

// Run runs the waiting changes of servers once they run, until ctx ends.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(interval)
	defer t.Stop()
	defer s.wg.Wait()
	for {
		s.applyWaiting(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// applyWaiting attempts the waiting changes of the servers that run, and looks for those of
// servers it doesn't know yet. Each server is attempted on its own, and not again while its
// last attempt runs, so that one that doesn't answer its console holds up only itself.
func (s *Service) applyWaiting(ctx context.Context) {
	servers, err := s.rt.List(ctx)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	known := s.waiting
	s.waiting = make(map[string]bool, len(servers))
	for _, srv := range servers {
		waits, ok := known[srv.ID]
		if ok {
			s.waiting[srv.ID] = waits
		}
		if !srv.Type.Proxy() && !s.applying[srv.ID] && (!ok || waits && srv.State == running) {
			s.applying[srv.ID] = true
			s.wg.Go(func() { s.apply(ctx, srv) })
		}
	}
}

// apply attempts the waiting changes of a server and notes whether changes still wait. It
// notes it before it unlocks the server, so that no change added in between is forgotten.
func (s *Service) apply(ctx context.Context, srv runtime.Server) {
	waits := true
	if unlock, err := s.locks.lock(ctx, srv.ID); err == nil {
		defer unlock()
		waits = s.applyLocked(ctx, srv)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waiting[srv.ID] = waits
	delete(s.applying, srv.ID)
}

// applyLocked runs the waiting changes of a running server, each until changeTimeout, and
// tells whether changes still wait.
func (s *Service) applyLocked(ctx context.Context, srv runtime.Server) bool {
	dir, err := s.rt.Data(ctx, srv.ID)
	if err != nil {
		return true
	}
	defer dir.Close()
	pending, err := readPending(dir)
	if err != nil || len(pending) == 0 || srv.State != running {
		return err != nil || len(pending) > 0
	}
	for i, change := range pending {
		changeCtx, cancel := context.WithTimeout(ctx, changeTimeout)
		var err error
		if bedrockWhitelist(change) {
			_, err = s.whitelistBedrock(changeCtx, srv.ID, dir, change)
		} else {
			_, err = s.rt.SendCommand(changeCtx, srv.ID, command(change))
		}
		cancel()
		if err != nil {
			slog.Warn("Can't change a player on a server that started", "server", srv.ID, "err", err)
			_ = writePending(dir, pending[i:]) // if it fails, the done changes run again, which changes nothing
			return true
		}
		slog.Info("Changed a player on a server that started", "server", srv.ID, "action", change.GetAction().Slug(), "player", change.GetName())
	}
	return writePending(dir, nil) != nil
}

// open finds a game server and opens its data directory.
func (s *Service) open(ctx context.Context, id string) (runtime.Server, *datadir.Dir, error) {
	if !runtime.ValidID(id) {
		return runtime.Server{}, nil, status.Error(codes.InvalidArgument, "invalid server ID")
	}
	srv, err := runtime.Find(ctx, s.rt, id)
	switch {
	case errors.Is(err, runtime.ErrNotFound):
		return srv, nil, status.Error(codes.NotFound, "Server not found.")
	case err != nil:
		return srv, nil, status.Error(codes.Internal, err.Error())
	case srv.Type.Proxy():
		return srv, nil, status.Error(codes.FailedPrecondition, "Players are managed on the game servers of a network.")
	}
	dir, err := s.rt.Data(ctx, id)
	if err != nil {
		return srv, nil, status.Error(codes.Internal, err.Error())
	}
	return srv, dir, nil
}

// command returns the console command of a valid change. Minecraft's own commands keep
// the lists in their files, also where plugins replace them.
func command(c *noryxv1.PlayerChange) string {
	return strings.TrimSpace("minecraft:" + commands[c.GetAction()] + " " + c.GetName() + " " + c.GetReason())
}
