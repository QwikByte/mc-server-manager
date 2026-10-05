package player

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/access"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/operation"
)

const (
	maxServers    = 500
	changeTimeout = 10 * time.Minute
)

type Handler struct {
	svc *Service
	ops *operation.Operations
}

func NewHandler(svc *Service, ops *operation.Operations) *Handler {
	return &Handler{svc: svc, ops: ops}
}

func (h *Handler) Register(mux access.Mux) {
	mux.Handle("POST /api/players/actions", access.SignedIn, h.change)
	mux.Handle("GET /api/players/lists", access.SignedIn, h.lists)
	mux.Handle("POST /api/networks/{id}/players/send", access.Everywhere(access.NetworksView), h.send)
}

// change kicks, bans, pardons, whitelists or makes operator a player on servers, or turns
// their whitelist on or off, as an operation that tells how it ended on each.
func (h *Handler) change(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action  string        `json:"action"`
		Name    string        `json:"name"`
		Reason  string        `json:"reason"`
		Servers []network.Ref `json:"servers"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	c := &noryxv1.PlayerChange{Action: noryxv1.ParsePlayerAction(req.Action), Name: strings.TrimSpace(req.Name), Reason: strings.TrimSpace(req.Reason)}
	if msg := c.Problem(); msg != "" {
		httpapi.WriteError(w, r, httpapi.Errorf(http.StatusBadRequest, "%s", msg))
		return
	}
	// Operators may run any command in the game.
	need := []access.Permission{access.PlayersManage}
	if c.GetAction() == noryxv1.PlayerAction_PLAYER_ACTION_OP || c.GetAction() == noryxv1.PlayerAction_PLAYER_ACTION_DEOP {
		need = append(need, access.ConsoleCommands)
	}
	if err := check(r, req.Servers, need...); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	logging.Note(r.Context(), slog.String("action", req.Action), slog.String("player", c.GetName()), slog.Int("servers", len(req.Servers)))
	subject := c.GetName()
	if c.GetAction().Global() {
		subject = strconv.Itoa(len(req.Servers))
	}
	h.ops.Run(w, r, operation.Spec{
		Kind: "players." + c.GetAction().Slug(), Subject: subject, Steps: []string{"servers"},
		Status: http.StatusOK, Timeout: changeTimeout, Category: logging.Players,
		Visible: func(g access.Grants) bool { return seesAll(g, req.Servers) },
	}, func(ctx context.Context) (any, error) {
		if err := h.svc.identify(ctx, c); err != nil {
			return nil, err
		}
		return map[string]any{"results": h.svc.Change(ctx, c, req.Servers)}, nil
	})
}

// lists returns the lists of the game servers of a network (?network=), of one server
// (?node=&server=), or of all that the user may see.
func (h *Handler) lists(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	grants, q := access.From(ctx), r.URL.Query()
	visible := func(ref network.Ref) bool { return grants.On(access.ServersView, ref.NodeID, ref.ServerID) }
	var servers []network.Ref
	var err error
	switch {
	case q.Has("network"):
		if !grants.Has(access.NetworksView) {
			err = access.Denied(access.NetworksView)
			break
		}
		var n network.Network
		if n, err = h.svc.networks.Get(ctx, q.Get("network")); err == nil {
			for _, b := range n.Backends {
				if visible(b.Ref) {
					servers = append(servers, b.Ref)
				}
			}
		}
	case q.Has("server"):
		ref := network.Ref{NodeID: q.Get("node"), ServerID: q.Get("server")}
		if !visible(ref) {
			err = access.Denied(access.ServersView)
		}
		servers = []network.Ref{ref}
	default:
		servers, err = h.svc.GameServers(ctx, visible)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, h.svc.Lists(ctx, servers))
}

// send sends a player to another server of a network.
func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Server string `json:"server"`
	}
	err := httpapi.ReadJSON(w, r, &req)
	var n network.Network
	if err == nil {
		n, err = h.svc.networks.Get(r.Context(), r.PathValue("id"))
	}
	if err == nil && !access.From(r.Context()).On(access.PlayersManage, n.Proxy.NodeID, n.Proxy.ServerID) {
		err = access.Denied(access.PlayersManage)
	}
	if err == nil {
		logging.Note(r.Context(), slog.String("name", n.Name), slog.String("player", req.Name), slog.String("server", req.Server))
		err = h.svc.Send(r.Context(), n, req.Name, req.Server)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// check checks that a request names up to maxServers different servers, on each of which
// the user has the permissions.
func check(r *http.Request, servers []network.Ref, need ...access.Permission) error {
	if len(servers) == 0 || len(servers) > maxServers {
		return httpapi.Errorf(http.StatusBadRequest, "Choose up to %d servers.", maxServers)
	}
	grants, seen := access.From(r.Context()), map[network.Ref]bool{}
	for _, s := range servers {
		if s.NodeID == "" || s.ServerID == "" || seen[s] {
			return httpapi.Errorf(http.StatusBadRequest, "Choose up to %d different servers.", maxServers)
		}
		seen[s] = true
		for _, p := range need {
			if !grants.On(p, s.NodeID, s.ServerID) {
				return access.Denied(p)
			}
		}
	}
	return nil
}

func seesAll(g access.Grants, servers []network.Ref) bool {
	return !slices.ContainsFunc(servers, func(s network.Ref) bool { return !g.On(access.ServersView, s.NodeID, s.ServerID) })
}
