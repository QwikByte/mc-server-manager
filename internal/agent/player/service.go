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
	rt runtime.Runtime

	mu sync.Mutex // guards the files of waiting changes and waiting
	// waiting tells which servers have changes that wait for them to run; servers the
	// agent didn't look at yet are missing.
	waiting map[string]bool
}

func NewService(rt runtime.Runtime) *Service { return &Service{rt: rt, waiting: map[string]bool{}} }

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
	s.mu.Lock()
	res.Pending, errs[4] = readPending(dir)
	s.mu.Unlock()
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
	if bedrockWhitelist(change) {
		return s.whitelistBedrock(ctx, srv, dir, change)
	}
	if srv.State == running {
		out, err := s.rt.SendCommand(ctx, srv.ID, command(change))
		if err != nil {
			return nil, status.Error(codes.Unavailable, err.Error())
		}
		return &noryxv1.ChangePlayerResponse{Output: strings.TrimSpace(formatting.ReplaceAllString(out, ""))}, nil
	}
	if change.GetAction() == noryxv1.PlayerAction_PLAYER_ACTION_KICK {
		return nil, status.Error(codes.FailedPrecondition, "The server doesn't run.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := addPending(dir, change); err != nil {
		return nil, err
	}
	s.waiting[srv.ID] = true
	return &noryxv1.ChangePlayerResponse{Pending: true}, nil
}

// Run runs the waiting changes of servers once they run, until ctx ends.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		s.applyWaiting(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

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
		if !srv.Type.Proxy() && (!ok || waits && srv.State == running) {
			waits = s.apply(ctx, srv)
		}
		s.waiting[srv.ID] = waits
	}
}

// apply runs the waiting changes of a running server, and tells whether changes still wait.
func (s *Service) apply(ctx context.Context, srv runtime.Server) bool {
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
		if _, err := s.rt.SendCommand(ctx, srv.ID, command(change)); err != nil {
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
