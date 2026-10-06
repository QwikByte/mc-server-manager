package network

import (
	"context"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
)

const (
	// maintenanceWait is how long the plugin may take to save a change; adding a player who
	// never joined asks Mojang for the player's ID first.
	maintenanceWait = 15 * time.Second
	maintenancePoll = 200 * time.Millisecond
)

// maintenanceCommands are the commands of the Maintenance plugin by kennytv for the changes.
var maintenanceCommands = map[noryxv1.MaintenanceChange]string{
	noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON:     "maintenance on",
	noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_OFF:    "maintenance off",
	noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD:    "maintenance add",
	noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_REMOVE: "maintenance remove",
}

func (s *Service) GetMaintenance(ctx context.Context, req *noryxv1.GetMaintenanceRequest) (*noryxv1.GetMaintenanceResponse, error) {
	srv, _, dir, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	m, err := readMaintenance(dir, srv.Type)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &noryxv1.GetMaintenanceResponse{Maintenance: m}, nil
}

func (s *Service) ChangeMaintenance(ctx context.Context, req *noryxv1.ChangeMaintenanceRequest) (*noryxv1.ChangeMaintenanceResponse, error) {
	change, player := req.GetChange(), req.GetPlayer()
	command := maintenanceCommands[change]
	listed := change == noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD || change == noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_REMOVE
	if command == "" || listed != noryxv1.ValidPlayerName(player) {
		return nil, status.Error(codes.InvalidArgument, "Choose a change, and the name of a player to add or remove.")
	}
	srv, _, dir, err := s.open(ctx, req.GetServerId())
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	if srv.State != noryxv1.ServerState_SERVER_STATE_RUNNING {
		return nil, status.Error(codes.FailedPrecondition, "Start the proxy first.")
	}
	// done tells whether the plugin's files show the change.
	done := func(m *noryxv1.Maintenance) bool {
		has := slices.ContainsFunc(m.GetPlayers(), func(p *noryxv1.MaintenancePlayer) bool { return strings.EqualFold(p.GetName(), player) })
		switch change {
		case noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ON:
			return m.GetEnabled()
		case noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_OFF:
			return !m.GetEnabled()
		case noryxv1.MaintenanceChange_MAINTENANCE_CHANGE_ADD:
			return has
		default:
			return !has
		}
	}
	m, err := readMaintenance(dir, srv.Type)
	switch {
	case err != nil:
		return nil, status.Error(codes.Internal, err.Error())
	case !m.GetInstalled():
		return nil, status.Error(codes.FailedPrecondition, "The proxy hasn't loaded the Maintenance plugin yet. Restart it.")
	case done(m):
		return &noryxv1.ChangeMaintenanceResponse{Maintenance: m}, nil
	}
	if _, err := s.rt.SendCommand(ctx, srv.ID, strings.TrimSpace(command+" "+player)); err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	ctx, cancel := context.WithTimeout(ctx, maintenanceWait)
	defer cancel()
	t := time.NewTicker(maintenancePoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, status.Error(codes.DeadlineExceeded, "The Maintenance plugin didn't save the change. The console of the proxy tells why.")
		case <-t.C:
		}
		if m, err := readMaintenance(dir, srv.Type); err == nil && done(m) {
			return &noryxv1.ChangeMaintenanceResponse{Maintenance: m}, nil
		}
	}
}

// readMaintenance reads the files of the Maintenance plugin in its folder: config.yml with
// whether maintenance is on, and WhitelistedPlayers.yml with the players who may join, by
// their ID. Without config.yml, the plugin isn't installed or wasn't loaded yet.
func readMaintenance(dir *datadir.Dir, typ noryxv1.ServerType) (*noryxv1.Maintenance, error) {
	folder := typ.MaintenanceFolder()
	data, err := dir.ReadOptional(filepath.FromSlash(path.Join(folder, "config.yml")))
	if err != nil || data == nil {
		return &noryxv1.Maintenance{}, err
	}
	var config struct {
		Enabled bool `yaml:"maintenance-enabled"`
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	data, err = dir.ReadOptional(filepath.FromSlash(path.Join(folder, "WhitelistedPlayers.yml")))
	var players map[string]string
	if err == nil {
		err = yaml.Unmarshal(data, &players)
	}
	if err != nil {
		return nil, err
	}
	m := &noryxv1.Maintenance{Installed: true, Enabled: config.Enabled}
	for uuid, name := range players {
		m.Players = append(m.Players, &noryxv1.MaintenancePlayer{Name: name, Uuid: uuid})
	}
	slices.SortFunc(m.Players, func(a, b *noryxv1.MaintenancePlayer) int {
		return strings.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
	})
	return m, nil
}
