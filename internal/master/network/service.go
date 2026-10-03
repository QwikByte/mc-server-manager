// Package network groups servers into networks: players join through a Velocity proxy,
// which forwards their verified identity to the game servers behind it. The master
// stores the networks and configures their servers through the node agents.
package network

import (
	"cmp"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/node"
)

const (
	queryTimeout = 10 * time.Second
	// configureTimeout covers recreating a running backend: a graceful stop and a start.
	configureTimeout = 3 * time.Minute
)

var (
	errNotFound        = httpapi.Errorf(http.StatusNotFound, "Network not found.")
	errBackendNotFound = httpapi.Errorf(http.StatusNotFound, "This server is not part of the network.")
	errProxyType       = httpapi.Errorf(http.StatusBadRequest, "Choose a Velocity proxy. BungeeCord is not supported because its player forwarding can be spoofed.")
	errBackendType     = httpapi.Errorf(http.StatusBadRequest, "Only Paper and Purpur servers can join a network, because they verify the players the proxy forwards.")

	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
)

// Ref points to a server on a node.
type Ref struct {
	NodeID   string `json:"nodeId"`
	ServerID string `json:"serverId"`
}

// Backend is a game server behind the proxy.
type Backend struct {
	Ref
	Name string `json:"name"` // what players use, e.g. /server lobby
}

type Network struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Proxy     Ref       `json:"proxy"`
	Backends  []Backend `json:"backends"` // players join the first one
	CreatedAt time.Time `json:"createdAt"`
	secret    string
}

// Nodes gives access to the nodes the servers of a network run on.
type Nodes interface {
	Get(ctx context.Context, id string) (node.Node, error)
	Conn(ctx context.Context, id string) (grpc.ClientConnInterface, error)
}

type Service struct {
	db    *sql.DB
	nodes Nodes
	mu    sync.Mutex // one change at a time, as changes reconfigure servers
}

func NewService(db *sql.DB, nodes Nodes) *Service { return &Service{db: db, nodes: nodes} }

func (s *Service) List(ctx context.Context) ([]Network, error) { return s.load(ctx, "") }

func (s *Service) Get(ctx context.Context, id string) (Network, error) {
	networks, err := s.load(ctx, id)
	if err == nil && len(networks) == 0 {
		err = errNotFound
	}
	if err != nil {
		return Network{}, err
	}
	return networks[0], nil
}

// Create sets up a network with its proxy and the server players join first.
func (s *Service) Create(ctx context.Context, name string, proxy, lobby Ref) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return Network{}, httpapi.Errorf(http.StatusBadRequest, "Enter a name with up to 64 characters.")
	}
	if srv, err := s.server(ctx, proxy); err != nil || srv.GetType() != mcsmv1.ServerType_SERVER_TYPE_VELOCITY {
		return Network{}, cmp.Or(err, errProxyType)
	}
	backend, err := s.backend(ctx, lobby, nil)
	if err != nil {
		return Network{}, err
	}
	n := Network{
		ID: strings.ToLower(rand.Text()), Name: name, Proxy: proxy, Backends: []Backend{backend},
		CreatedAt: time.Now(), secret: rand.Text(),
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO networks (id, name, proxy_node_id, proxy_server_id, forwarding_secret, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			n.ID, n.Name, proxy.NodeID, proxy.ServerID, n.secret, n.CreatedAt.Unix()); err != nil {
			return err
		}
		return insertBackend(ctx, tx, n.ID, backend)
	})
	if err != nil {
		return Network{}, conflict(err, n.Name)
	}
	return n, s.apply(ctx, n)
}

// AddBackend adds a game server to a network.
func (s *Service) AddBackend(ctx context.Context, id string, ref Ref) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.Get(ctx, id)
	if err != nil {
		return n, err
	}
	backend, err := s.backend(ctx, ref, n.Backends)
	if err != nil {
		return n, err
	}
	if err := insertBackend(ctx, s.db, n.ID, backend); err != nil {
		return n, conflict(err, n.Name)
	}
	n.Backends = append(n.Backends, backend)
	return n, s.apply(ctx, n)
}

// RemoveBackend makes a server standalone again and removes it from the network.
func (s *Service) RemoveBackend(ctx context.Context, id, serverID string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, i, err := s.getBackend(ctx, id, serverID)
	if err != nil {
		return n, err
	}
	if len(n.Backends) == 1 {
		return n, httpapi.Errorf(http.StatusConflict, "A network needs at least one server. Delete the network instead.")
	}
	// The server is reset first: one that still trusts the proxy must not be forgotten.
	if err := s.reset(ctx, n.Backends[i].Ref); err != nil {
		return n, httpapi.Errorf(http.StatusBadGateway, "%s could not be made standalone again: %s", n.Backends[i].Name, status.Convert(err).Message())
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM network_backends WHERE network_id = ? AND server_id = ?`, n.ID, serverID); err != nil {
		return n, err
	}
	n.Backends = slices.Delete(n.Backends, i, i+1)
	return n, s.configureProxy(context.WithoutCancel(ctx), n)
}

// MakeDefault makes the given backend the server players join first.
func (s *Service) MakeDefault(ctx context.Context, id, serverID string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, i, err := s.getBackend(ctx, id, serverID)
	if err != nil {
		return n, err
	}
	n.Backends = append([]Backend{n.Backends[i]}, slices.Delete(slices.Clone(n.Backends), i, i+1)...)
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		for position, b := range n.Backends {
			if _, err := tx.ExecContext(ctx, `UPDATE network_backends SET position = ? WHERE network_id = ? AND server_id = ?`,
				position, n.ID, b.ServerID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	return n, s.configureProxy(context.WithoutCancel(ctx), n)
}

// Delete makes all servers standalone again and removes the network. The proxy keeps
// running without backends.
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	ctx = context.WithoutCancel(ctx)
	for _, b := range n.Backends {
		if err := s.reset(ctx, b.Ref); err != nil {
			return httpapi.Errorf(http.StatusBadGateway, "The network was not deleted, because %s could not be made standalone again: %s", b.Name, status.Convert(err).Message())
		}
	}
	if err := ignoreMissing(s.configure(ctx, n.Proxy, n.secret, nil, false)); err != nil {
		return httpapi.Errorf(http.StatusBadGateway, "The network was not deleted, because its proxy could not be updated: %s", status.Convert(err).Message())
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM networks WHERE id = ?`, n.ID)
	return err
}

// CheckRemovable fails if a server is part of a network, which would break without it.
func (s *Service) CheckRemovable(ctx context.Context, nodeID, serverID string) error {
	var name string
	err := s.db.QueryRowContext(ctx, `
		SELECT name FROM networks WHERE proxy_node_id = ?1 AND proxy_server_id = ?2
		UNION SELECT n.name FROM network_backends b JOIN networks n ON n.id = b.network_id
		WHERE b.node_id = ?1 AND b.server_id = ?2`, nodeID, serverID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return httpapi.Errorf(http.StatusConflict, "This server is part of the network %q. Remove it from the network first.", name)
}

// Move points the network of a server that moved to another node at its new place, and
// configures the network again, as the proxy reaches the server at another address. A
// server without a network needs nothing.
func (s *Service) Move(ctx context.Context, serverID, from, to string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var id string
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, query := range []string{
			`UPDATE networks SET proxy_node_id = ?1 WHERE proxy_node_id = ?2 AND proxy_server_id = ?3 RETURNING id`,
			`UPDATE network_backends SET node_id = ?1 WHERE node_id = ?2 AND server_id = ?3 RETURNING network_id`,
		} {
			if err := tx.QueryRowContext(ctx, query, to, from, serverID).Scan(&id); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		return nil
	})
	if err != nil || id == "" {
		return err
	}
	n, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.apply(ctx, n)
}

// Apply configures all servers of a network again, e.g. after a node was offline.
func (s *Service) Apply(ctx context.Context, id string) (Network, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.Get(ctx, id)
	if err != nil {
		return n, err
	}
	return n, s.apply(ctx, n)
}

// apply configures the backends first, so that they accept the proxy before it sends
// players to them, then the proxy. Servers only restart if their configuration changed.
func (s *Service) apply(ctx context.Context, n Network) error {
	ctx = context.WithoutCancel(ctx) // finish even if the client goes away
	for _, b := range n.Backends {
		if err := s.configure(ctx, b.Ref, n.secret, nil, b.NodeID == n.Proxy.NodeID); err != nil {
			return applyFailed(b.Name, err)
		}
	}
	return s.configureProxy(ctx, n)
}

func (s *Service) configureProxy(ctx context.Context, n Network) error {
	backends := make([]*mcsmv1.NetworkBackend, 0, len(n.Backends))
	for _, b := range n.Backends {
		target, err := s.target(ctx, n.Proxy, b)
		if err != nil {
			return applyFailed(b.Name, err)
		}
		backends = append(backends, target)
	}
	if err := s.configure(ctx, n.Proxy, n.secret, backends, false); err != nil {
		return applyFailed("the proxy", err)
	}
	return nil
}

// target tells the proxy how to reach a backend: on its own node by server ID, as the
// agent knows the local route, otherwise at the node's address and the server's port.
func (s *Service) target(ctx context.Context, proxy Ref, b Backend) (*mcsmv1.NetworkBackend, error) {
	if b.NodeID == proxy.NodeID {
		return &mcsmv1.NetworkBackend{Name: b.Name, Target: &mcsmv1.NetworkBackend_ServerId{ServerId: b.ServerID}}, nil
	}
	n, err := s.nodes.Get(ctx, b.NodeID)
	if err != nil {
		return nil, err
	}
	srv, err := s.server(ctx, b.Ref)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(n.Address)
	if err != nil {
		return nil, err
	}
	address := net.JoinHostPort(host, strconv.Itoa(int(srv.GetPort())))
	return &mcsmv1.NetworkBackend{Name: b.Name, Target: &mcsmv1.NetworkBackend_Address{Address: address}}, nil
}

// configure gives a server its role in a network; proxyOnNode tells a backend that its
// proxy runs on the same node.
func (s *Service) configure(ctx context.Context, ref Ref, secret string, backends []*mcsmv1.NetworkBackend, proxyOnNode bool) error {
	ctx, cancel := context.WithTimeout(ctx, configureTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return err
	}
	_, err = mcsmv1.NewServerServiceClient(conn).ConfigureNetwork(ctx, &mcsmv1.ConfigureNetworkRequest{
		Id: ref.ServerID, ForwardingSecret: secret, Backends: backends, ProxyOnNode: proxyOnNode,
	})
	return err
}

// reset makes a game server standalone again. A deleted server needs no reset.
func (s *Service) reset(ctx context.Context, ref Ref) error {
	return ignoreMissing(s.configure(ctx, ref, "", nil, false))
}

// server looks up a server on its node.
func (s *Service) server(ctx context.Context, ref Ref) (*mcsmv1.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, ref.NodeID)
	if err != nil {
		return nil, err
	}
	res, err := mcsmv1.NewServerServiceClient(conn).ListServers(ctx, &mcsmv1.ListServersRequest{})
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(res.GetServers(), func(srv *mcsmv1.Server) bool { return srv.GetId() == ref.ServerID })
	if i < 0 {
		return nil, httpapi.Errorf(http.StatusNotFound, "Server not found.")
	}
	return res.GetServers()[i], nil
}

// backend checks that a server can join a network and names it uniquely.
func (s *Service) backend(ctx context.Context, ref Ref, existing []Backend) (Backend, error) {
	srv, err := s.server(ctx, ref)
	if err != nil {
		return Backend{}, err
	}
	if !slices.Contains([]mcsmv1.ServerType{mcsmv1.ServerType_SERVER_TYPE_PAPER, mcsmv1.ServerType_SERVER_TYPE_PURPUR}, srv.GetType()) {
		return Backend{}, errBackendType
	}
	return Backend{Ref: ref, Name: backendName(srv.GetName(), existing)}, nil
}

func (s *Service) getBackend(ctx context.Context, id, serverID string) (Network, int, error) {
	n, err := s.Get(ctx, id)
	if err != nil {
		return n, -1, err
	}
	i := slices.IndexFunc(n.Backends, func(b Backend) bool { return b.ServerID == serverID })
	if i < 0 {
		return n, -1, errBackendNotFound
	}
	return n, i, nil
}

// load reads one network, or all networks when id is empty.
func (s *Service) load(ctx context.Context, id string) ([]Network, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, proxy_node_id, proxy_server_id, forwarding_secret, created_at
		FROM networks WHERE ? IN ('', id) ORDER BY name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	networks := []Network{}
	index := map[string]int{}
	for rows.Next() {
		n := Network{Backends: []Backend{}}
		var createdAt int64
		if err := rows.Scan(&n.ID, &n.Name, &n.Proxy.NodeID, &n.Proxy.ServerID, &n.secret, &createdAt); err != nil {
			return nil, err
		}
		n.CreatedAt = time.Unix(createdAt, 0)
		index[n.ID] = len(networks)
		networks = append(networks, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	backends, err := s.db.QueryContext(ctx, `
		SELECT network_id, node_id, server_id, name FROM network_backends
		WHERE ? IN ('', network_id) ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer backends.Close()
	for backends.Next() {
		var networkID string
		var b Backend
		if err := backends.Scan(&networkID, &b.NodeID, &b.ServerID, &b.Name); err != nil {
			return nil, err
		}
		if i, ok := index[networkID]; ok {
			networks[i].Backends = append(networks[i].Backends, b)
		}
	}
	return networks, backends.Err()
}

// inTx runs fn in a transaction that is committed if fn succeeds.
func (s *Service) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertBackend(ctx context.Context, db execer, networkID string, b Backend) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO network_backends (network_id, node_id, server_id, name, position)
		VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM network_backends WHERE network_id = ?))`,
		networkID, b.NodeID, b.ServerID, b.Name, networkID)
	return err
}

// backendName derives the name players use for a server, e.g. "Survival 2" becomes
// "survival-2", and keeps it unique within the network.
func backendName(serverName string, existing []Backend) string {
	slug := nonSlug.ReplaceAllString(strings.ToLower(serverName), "-")
	base := cmp.Or(strings.Trim(slug[:min(len(slug), 28)], "-"), "server")
	if base == "try" { // reserved for Velocity's list of servers to try
		base = "try-server"
	}
	name := base
	for i := 2; slices.ContainsFunc(existing, func(b Backend) bool { return b.Name == name }); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

func conflict(err error, name string) error {
	switch msg := err.Error(); {
	case strings.Contains(msg, "networks.name"):
		return httpapi.Errorf(http.StatusConflict, "A network named %q already exists.", name)
	case strings.Contains(msg, "proxy_server_id"):
		return httpapi.Errorf(http.StatusConflict, "This proxy already serves a network.")
	case strings.Contains(msg, "network_backends.server_id"):
		return httpapi.Errorf(http.StatusConflict, "This server already belongs to a network.")
	}
	return err
}

func applyFailed(what string, err error) error {
	return httpapi.Errorf(http.StatusBadGateway, "The change was saved, but %s could not be configured: %s Apply the network again once all nodes are online.",
		what, strings.TrimSuffix(status.Convert(err).Message(), ".")+".")
}

// ignoreMissing ignores that the agent does not know a server, e.g. as it was deleted.
func ignoreMissing(err error) error {
	if status.Code(err) == codes.NotFound {
		return nil
	}
	return err
}
