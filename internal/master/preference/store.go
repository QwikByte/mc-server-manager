// Package preference keeps each user's preferences of the panel: the layout of their
// overview and the servers they pinned. They follow the user into every browser.
package preference

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	maxWidgets = 32
	maxColumns = 3
	maxPinned  = 20
)

var (
	widgetPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	// idPattern matches the IDs of nodes and servers, which are random base32 in lowercase.
	idPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)
)

// Preferences are what a user chose for the panel.
type Preferences struct {
	// Dashboard is the order of the widgets of the overview; empty shows the panel's default layout.
	Dashboard []Widget `json:"dashboard"`
	// Pinned are the servers the user pinned, in their order.
	Pinned []Server `json:"pinned"`
}

// Widget is a widget of the overview, which spans 1 to 3 columns of its grid.
type Widget struct {
	ID      string `json:"id"`
	Columns int    `json:"columns"`
	Hidden  bool   `json:"hidden,omitempty"`
}

// Server is a server on a node.
type Server struct {
	NodeID   string `json:"nodeId"`
	ServerID string `json:"serverId"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Get returns the preferences of a user. Those the user never set are empty, not nil.
func (s *Store) Get(ctx context.Context, userID int64) (Preferences, error) {
	p := Preferences{Dashboard: []Widget{}, Pinned: []Server{}}
	var widgets string
	err := s.db.QueryRowContext(ctx, `SELECT widgets FROM dashboards WHERE user_id = ?`, userID).Scan(&widgets)
	switch {
	case err == nil:
		err = json.Unmarshal([]byte(widgets), &p.Dashboard)
	case errors.Is(err, sql.ErrNoRows):
		err = nil
	}
	if err != nil {
		return p, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT node_id, server_id FROM pinned_servers WHERE user_id = ? ORDER BY position`, userID)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var srv Server
		if err := rows.Scan(&srv.NodeID, &srv.ServerID); err != nil {
			return p, err
		}
		p.Pinned = append(p.Pinned, srv)
	}
	return p, rows.Err()
}

// SetDashboard stores the layout of a user's overview; an empty one brings back the default.
func (s *Store) SetDashboard(ctx context.Context, userID int64, widgets []Widget) error {
	if err := checkDashboard(widgets); err != nil {
		return err
	}
	if len(widgets) == 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM dashboards WHERE user_id = ?`, userID)
		return err
	}
	data, err := json.Marshal(widgets)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO dashboards (user_id, widgets) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET widgets = excluded.widgets`, userID, string(data))
	return err
}

// SetPinned replaces the servers a user pinned, keeping their order. Servers of nodes that
// were removed in the meantime are left out, as the pins of removed nodes go with them.
func (s *Store) SetPinned(ctx context.Context, userID int64, servers []Server) error {
	if err := checkPinned(servers); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	if _, err := tx.ExecContext(ctx, `DELETE FROM pinned_servers WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for i, srv := range servers {
		if _, err := tx.ExecContext(ctx, `INSERT INTO pinned_servers (user_id, position, node_id, server_id)
			SELECT ?, ?, id, ? FROM nodes WHERE id = ?`, userID, i, srv.ServerID, srv.NodeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Forget unpins a deleted server for every user.
func (s *Store) Forget(ctx context.Context, nodeID, serverID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pinned_servers WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	return err
}

// Move keeps a server that moved to another node pinned. Users who pinned it at both places
// keep the pin at its new one.
func (s *Store) Move(ctx context.Context, serverID, from, to string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE OR IGNORE pinned_servers SET node_id = ? WHERE node_id = ? AND server_id = ?`, to, from, serverID)
	if err == nil {
		err = s.Forget(ctx, from, serverID)
	}
	return err
}

func checkDashboard(widgets []Widget) error {
	if len(widgets) > maxWidgets {
		return httpapi.Errorf(http.StatusBadRequest, "The overview has up to %d widgets.", maxWidgets)
	}
	seen := make(map[string]bool, len(widgets))
	for _, w := range widgets {
		switch {
		case !widgetPattern.MatchString(w.ID):
			return httpapi.Errorf(http.StatusBadRequest, "Widgets have IDs of up to 32 lowercase letters, digits and -, starting with a letter.")
		case seen[w.ID]:
			return httpapi.Errorf(http.StatusBadRequest, "The overview has the widget %q twice.", w.ID)
		case w.Columns < 1 || w.Columns > maxColumns:
			return httpapi.Errorf(http.StatusBadRequest, "Widgets span 1 to %d columns.", maxColumns)
		}
		seen[w.ID] = true
	}
	return nil
}

func checkPinned(servers []Server) error {
	if len(servers) > maxPinned {
		return httpapi.Errorf(http.StatusBadRequest, "Pin up to %d servers.", maxPinned)
	}
	seen := make(map[Server]bool, len(servers))
	for _, srv := range servers {
		switch {
		case !idPattern.MatchString(srv.NodeID) || !idPattern.MatchString(srv.ServerID):
			return httpapi.Errorf(http.StatusBadRequest, "Pinned servers need the IDs of their node and their own.")
		case seen[srv]:
			return httpapi.Errorf(http.StatusBadRequest, "A server is pinned twice.")
		}
		seen[srv] = true
	}
	return nil
}
