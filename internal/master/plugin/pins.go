package plugin

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// maxPins limits the projects a server keeps at their version, as many as it can have files.
const maxPins = 1000

// pins are the projects that servers keep at their version, which updates of all plugins leave
// out. Servers live on their agents, so the master keeps them by node and server.
type pins struct{ db *sql.DB }

// all returns the pinned projects of each server that has some, or of one server if set.
func (p pins) all(ctx context.Context, only *Ref) (map[Ref]map[string]bool, error) {
	query, args := `SELECT node_id, server_id, project_id FROM plugin_pins`, []any{}
	if only != nil {
		query, args = query+` WHERE node_id = ? AND server_id = ?`, []any{only.NodeID, only.ServerID}
	}
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := map[Ref]map[string]bool{}
	for rows.Next() {
		var ref Ref
		var project string
		if err := rows.Scan(&ref.NodeID, &ref.ServerID, &project); err != nil {
			return nil, err
		}
		if all[ref] == nil {
			all[ref] = map[string]bool{}
		}
		all[ref][project] = true
	}
	return all, rows.Err()
}

// of returns the pinned projects of a server.
func (p pins) of(ctx context.Context, ref Ref) (map[string]bool, error) {
	all, err := p.all(ctx, &ref)
	return all[ref], err
}

// set pins a project of a server, or unpins it.
func (p pins) set(ctx context.Context, ref Ref, project string, pinned bool) error {
	if !ValidProjectID(project) {
		return httpapi.Errorf(http.StatusBadRequest, "invalid project ID")
	}
	if !pinned {
		_, err := p.db.ExecContext(ctx, `DELETE FROM plugin_pins WHERE node_id = ? AND server_id = ? AND project_id = ?`, ref.NodeID, ref.ServerID, project)
		return err
	}
	var count int
	if err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM plugin_pins WHERE node_id = ? AND server_id = ?`, ref.NodeID, ref.ServerID).Scan(&count); err != nil {
		return err
	}
	if count >= maxPins {
		return httpapi.Errorf(http.StatusConflict, "A server can keep up to %d projects at their version.", maxPins)
	}
	_, err := p.db.ExecContext(ctx, `INSERT OR IGNORE INTO plugin_pins (node_id, server_id, project_id) VALUES (?, ?, ?)`, ref.NodeID, ref.ServerID, project)
	return err
}

// Forget deletes the pins of a deleted server.
func (s *Service) Forget(ctx context.Context, nodeID, serverID string) error {
	_, err := s.pins.db.ExecContext(ctx, `DELETE FROM plugin_pins WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	return err
}

// Move keeps the pins of a server that moved to another node.
func (s *Service) Move(ctx context.Context, serverID, from, to string) error {
	_, err := s.pins.db.ExecContext(ctx, `UPDATE OR REPLACE plugin_pins SET node_id = ? WHERE node_id = ? AND server_id = ?`, to, from, serverID)
	return err
}
