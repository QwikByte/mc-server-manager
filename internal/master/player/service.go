// Package player manages the players of game servers through their agents: kicks, bans,
// the whitelist and operators on many servers at once, the lists of a network or of all
// servers, and sending players to another server of their network.
package player

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/node"
	"github.com/QwikByte/noryx/internal/master/operation"
)

const queryTimeout = 10 * time.Second

// Nodes gives access to the agents of the nodes.
type Nodes interface {
	List(ctx context.Context) ([]node.Node, error)
	Conn(ctx context.Context, id string) (grpc.ClientConnInterface, error)
}

// Networks looks up networks, whose proxies send players between their servers.
type Networks interface {
	Get(ctx context.Context, id string) (network.Network, error)
}

type Service struct {
	nodes    Nodes
	networks Networks
	bedrock  BedrockPlayers
}

func NewService(nodes Nodes, networks Networks, bedrock BedrockPlayers) *Service {
	return &Service{nodes: nodes, networks: networks, bedrock: bedrock}
}

// Result tells how a change ended on a server.
type Result struct {
	network.Ref
	// Pending tells that the server doesn't run and changes the player once it does.
	Pending bool   `json:"pending,omitempty"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Change changes a player on servers: at once on those that run, and on the others once
// they do.
func (s *Service) Change(ctx context.Context, c *noryxv1.PlayerChange, servers []network.Ref) []Result {
	results := make([]Result, len(servers))
	failed := func(i int, err error) { results[i] = Result{Ref: servers[i], Error: httpapi.Message(err)} }
	operation.Each(ctx, nodesOf(servers), func(ctx context.Context, i int) {
		var res *noryxv1.ChangePlayerResponse
		err := s.call(ctx, servers[i].NodeID, func(ctx context.Context, conn grpc.ClientConnInterface) (err error) {
			res, err = noryxv1.NewPlayerServiceClient(conn).ChangePlayer(ctx, &noryxv1.ChangePlayerRequest{ServerId: servers[i].ServerID, Change: c})
			return err
		})
		if err != nil {
			failed(i, err)
			return
		}
		results[i] = Result{Ref: servers[i], Pending: res.GetPending(), Output: res.GetOutput()}
	}, failed)
	return results
}

// Listed is a player in the lists of servers.
type Listed struct {
	Name string `json:"name"`
	UUID string `json:"uuid,omitempty"`
	// Reason, Since, Until and Source tell about the newest ban; Until is when it ends, if
	// it is temporary.
	Reason string     `json:"reason,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	Until  *time.Time `json:"until,omitempty"`
	Source string     `json:"source,omitempty"`
	// Servers are those whose list has the player.
	Servers []network.Ref `json:"servers"`
}

// ServerState is what the lists of a server don't tell: whether its whitelist is on, and
// the changes that wait for it to run.
type ServerState struct {
	network.Ref
	WhitelistEnabled bool      `json:"whitelistEnabled"`
	Pending          []Pending `json:"pending"`
	// Error tells why the lists of the server are missing.
	Error string `json:"error,omitempty"`
}

// Pending is a change that waits for a server to run.
type Pending struct {
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Lists are the lists of servers, joined by player.
type Lists struct {
	Servers     []ServerState `json:"servers"`
	Banned      []Listed      `json:"banned"`
	Whitelisted []Listed      `json:"whitelisted"`
	Operators   []Listed      `json:"operators"`
}

// Lists returns who is banned, whitelisted and operator on the given servers.
func (s *Service) Lists(ctx context.Context, servers []network.Ref) Lists {
	got := make([]*noryxv1.GetPlayerListsResponse, len(servers))
	out := Lists{Servers: make([]ServerState, len(servers))}
	operation.Each(ctx, nodesOf(servers), func(ctx context.Context, i int) {
		out.Servers[i] = ServerState{Ref: servers[i], Pending: []Pending{}}
		err := s.call(ctx, servers[i].NodeID, func(ctx context.Context, conn grpc.ClientConnInterface) (err error) {
			got[i], err = noryxv1.NewPlayerServiceClient(conn).GetPlayerLists(ctx, &noryxv1.GetPlayerListsRequest{ServerId: servers[i].ServerID})
			return err
		})
		if err != nil {
			out.Servers[i].Error = httpapi.Message(err)
		}
	}, nil)
	var banned, whitelisted, operators joined
	for i, res := range got {
		ref := servers[i]
		banned.add(ref, res.GetBanned())
		whitelisted.add(ref, res.GetWhitelisted())
		operators.add(ref, res.GetOperators())
		out.Servers[i].WhitelistEnabled = res.GetWhitelistEnabled()
		for _, p := range res.GetPending() {
			out.Servers[i].Pending = append(out.Servers[i].Pending, Pending{p.GetAction().Slug(), p.GetName(), p.GetReason()})
		}
	}
	out.Banned, out.Whitelisted, out.Operators = banned.sorted(), whitelisted.sorted(), operators.sorted()
	return out
}

// joined joins the entries of lists by the name of the player.
type joined struct {
	list  []Listed
	index map[string]int
}

func (j *joined) add(ref network.Ref, players []*noryxv1.ListedPlayer) {
	if j.index == nil {
		j.index = map[string]int{}
	}
	for _, p := range players {
		key := strings.ToLower(p.GetName())
		i, ok := j.index[key]
		if !ok {
			i, j.index[key] = len(j.list), len(j.list)
			j.list = append(j.list, Listed{Name: p.GetName(), UUID: p.GetUuid()})
		}
		l := &j.list[i]
		l.Servers = append(l.Servers, ref)
		if since := time.Unix(p.GetCreatedUnix(), 0); p.GetCreatedUnix() > 0 && (l.Since == nil || since.After(*l.Since)) {
			l.Since, l.Reason, l.Source, l.Until = &since, p.GetReason(), p.GetSource(), nil
			if p.GetExpiresUnix() > 0 {
				until := time.Unix(p.GetExpiresUnix(), 0)
				l.Until = &until
			}
		}
	}
}

func (j *joined) sorted() []Listed {
	list := append([]Listed{}, j.list...)
	slices.SortFunc(list, func(a, b Listed) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return list
}

// GameServers returns the game servers of all nodes that the filter lets through; nodes
// that can't be reached are left out.
func (s *Service) GameServers(ctx context.Context, keep func(network.Ref) bool) ([]network.Ref, error) {
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, err
	}
	perNode := make([][]network.Ref, len(nodes))
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	operation.Each(ctx, ids, func(ctx context.Context, i int) {
		if nodes[i].EnrolledAt == nil {
			return
		}
		var res *noryxv1.ListServersResponse
		err := s.call(ctx, ids[i], func(ctx context.Context, conn grpc.ClientConnInterface) (err error) {
			res, err = noryxv1.NewServerServiceClient(conn).ListServers(ctx, &noryxv1.ListServersRequest{})
			return err
		})
		if err != nil {
			return
		}
		for _, srv := range res.GetServers() {
			if ref := (network.Ref{NodeID: ids[i], ServerID: srv.GetId()}); !srv.GetType().Proxy() && keep(ref) {
				perNode[i] = append(perNode[i], ref)
			}
		}
	}, nil)
	return slices.Concat(perNode...), nil
}

// Send sends a player to a server of a network with the proxy's send command, which
// Velocity has, and BungeeCord with its module cmd_send. The agent tells if BungeeCord
// couldn't send the player.
func (s *Service) Send(ctx context.Context, n network.Network, player, server string) error {
	command, alone := n.SendCommand(player, server)
	switch {
	case !noryxv1.ValidPlayerName(player):
		return httpapi.Errorf(http.StatusBadRequest, "Enter the name of a player: up to 16 letters, digits and underscores.")
	case !slices.ContainsFunc(n.Backends, func(b network.Backend) bool { return b.Name == server }):
		return httpapi.Errorf(http.StatusBadRequest, "Choose a server of the network.")
	case !alone:
		return httpapi.Errorf(http.StatusBadRequest, "The proxy can't send %s, as its send command reads this name as other players too.", player)
	}
	return s.call(ctx, n.Proxy.NodeID, func(ctx context.Context, conn grpc.ClientConnInterface) error {
		_, err := noryxv1.NewServerServiceClient(conn).SendCommand(ctx, &noryxv1.SendCommandRequest{Id: n.Proxy.ServerID, Command: command})
		return err
	})
}

// call calls the agent of a node.
func (s *Service) call(ctx context.Context, nodeID string, fn func(ctx context.Context, conn grpc.ClientConnInterface) error) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, nodeID)
	if err == nil {
		err = fn(ctx, conn)
	}
	if status.Code(err) == codes.Unimplemented {
		err = httpapi.Errorf(http.StatusNotImplemented, "Update the agent of this node to manage players.")
	}
	return err
}

func nodesOf(servers []network.Ref) []string {
	nodes := make([]string, len(servers))
	for i, s := range servers {
		nodes[i] = s.NodeID
	}
	return nodes
}
