package network

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/operation"
	"github.com/QwikByte/noryx/internal/master/plugin"
)

// maintenanceProject is the Maintenance plugin by kennytv on Modrinth, which runs on
// Velocity, BungeeCord and Waterfall.
const maintenanceProject = "VCAqN1ln"

const (
	// pluginTimeout is how long a proxy may take to load the plugin after it started.
	pluginTimeout = time.Minute
	// maintenanceWait is how long the agent waits for the plugin to save a change.
	maintenanceWait = 15 * time.Second
)

// Maintenance is the maintenance of a network, as the Maintenance plugin of its proxy keeps it.
type Maintenance struct {
	// Installed tells that the proxy loaded the plugin.
	Installed bool `json:"installed"`
	Enabled   bool `json:"enabled"`
	// Players may join during maintenance.
	Players      []MaintenancePlayer `json:"players"`
	ProxyRunning bool                `json:"proxyRunning"`
}

type MaintenancePlayer struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

// Maintenance tells whether a network is in maintenance.
func (s *Service) Maintenance(ctx context.Context, n Network) (Maintenance, error) {
	proxy, err := s.server(ctx, n.Proxy)
	if err != nil {
		return Maintenance{}, err
	}
	var res *noryxv1.GetMaintenanceResponse
	err = s.proxy(ctx, n, func(ctx context.Context, c noryxv1.ProxyServiceClient) (err error) {
		res, err = c.GetMaintenance(ctx, &noryxv1.GetMaintenanceRequest{ServerId: n.Proxy.ServerID})
		return err
	})
	return maintenanceOf(res.GetMaintenance(), proxy.GetState() == noryxv1.ServerState_SERVER_STATE_RUNNING), err
}

// SetMaintenance turns maintenance on or off. The first time, the plugin is installed on
// the proxy, which restarts to load it.
func (s *Service) SetMaintenance(ctx context.Context, n Network, on bool) (Maintenance, error) {
	ctx = context.WithoutCancel(ctx)
	m, err := s.Maintenance(ctx, n)
	switch {
	case err != nil:
		return m, err
	case !m.ProxyRunning:
		return m, httpapi.Errorf(http.StatusConflict, "Start the proxy of the network first.")
	case !m.Installed && !on:
		return m, nil
	case !m.Installed:
		operation.Step(ctx, "plugin")
		if err := s.mods.Ensure(ctx, plugin.Ref(n.Proxy), maintenanceProject); err != nil {
			return m, err
		}
		operation.Step(ctx, "proxy-restart")
		if err := s.restart(ctx, []Backend{{Ref: n.Proxy, Name: "The proxy"}}); err != nil {
			return m, err
		}
		// The plugin creates its files once the proxy loaded it.
		err := waitFor(ctx, pluginTimeout, func(ctx context.Context) (bool, error) {
			m, err := s.Maintenance(ctx, n)
			return m.Installed, err
		})
		if err != nil {
			return m, timedOut(err, "The proxy didn't load the Maintenance plugin. Its console tells why.")
		}
	}
	operation.Step(ctx, "maintenance")
	change := noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_OFF
	if on {
		change = noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON
	}
	return s.changeMaintenance(ctx, n, change, "")
}

// ChangeMaintenancePlayer adds or removes a player who may join during maintenance.
func (s *Service) ChangeMaintenancePlayer(ctx context.Context, n Network, add bool, player string) (Maintenance, error) {
	change := noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_REMOVE
	if add {
		change = noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD
	}
	return s.changeMaintenance(ctx, n, change, player)
}

func (s *Service) changeMaintenance(ctx context.Context, n Network, change noryxv1.MaintenanceChange, player string) (Maintenance, error) {
	if player != "" && !noryxv1.ValidPlayerName(player) {
		return Maintenance{}, httpapi.Errorf(http.StatusBadRequest, "Enter the name of a player: up to 16 letters, digits and underscores.")
	}
	var res *noryxv1.ChangeMaintenanceResponse
	err := s.proxy(ctx, n, func(ctx context.Context, c noryxv1.ProxyServiceClient) (err error) {
		res, err = c.ChangeMaintenance(ctx, &noryxv1.ChangeMaintenanceRequest{ServerId: n.Proxy.ServerID, Change: change, Player: player})
		return err
	})
	return maintenanceOf(res.GetMaintenance(), true), err
}

// proxy calls the ProxyService of the agent of a network's proxy, with time for the plugin to
// look up a player who never joined.
func (s *Service) proxy(ctx context.Context, n Network, call func(ctx context.Context, c noryxv1.ProxyServiceClient) error) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout+maintenanceWait)
	defer cancel()
	conn, err := s.nodes.Conn(ctx, n.Proxy.NodeID)
	if err == nil {
		err = call(ctx, noryxv1.NewProxyServiceClient(conn))
	}
	if status.Code(err) == codes.Unimplemented {
		err = httpapi.Errorf(http.StatusNotImplemented, "Update the agent of the proxy's node for maintenance.")
	}
	return err
}

func maintenanceOf(m *noryxv1.Maintenance, proxyRunning bool) Maintenance {
	out := Maintenance{Installed: m.GetInstalled(), Enabled: m.GetEnabled(), Players: []MaintenancePlayer{}, ProxyRunning: proxyRunning}
	for _, p := range m.GetPlayers() {
		out.Players = append(out.Players, MaintenancePlayer{p.GetName(), p.GetUuid()})
	}
	return out
}
