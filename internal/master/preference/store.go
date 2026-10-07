// Package preference keeps each user's preferences of the panel: the layout of their
// overview, the servers they pinned and settings such as the colour theme. They follow the
// user into every browser.
package preference

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"

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

// settings are the keys of Settings and the values each takes, which the panel knows too
// (web/src/features/preferences/api.ts).
var settings = map[string][]string{
	"theme":   {"light", "dark", "system"},
	"accent":  {"emerald", "blue", "violet", "graphite"},
	"density": {"comfortable", "compact"},
	"clock":   {"12h", "24h"},
	// How lists of servers are shown where their address doesn't say.
	"serverView":  {"grid", "table"},
	"serverSort":  {"name", "state", "players", "cpu", "memory", "node"},
	"serverOrder": {"asc", "desc"},
	"serverGroup": {"none", "network", "node", "type", "tag"},
}

// Preferences are what a user chose for the panel.
type Preferences struct {
	// Dashboard is the order of the widgets of the overview; empty shows the panel's default layout.
	Dashboard []Widget `json:"dashboard"`
	// Pinned are the servers the user pinned, in their order.
	Pinned []Server `json:"pinned"`
	// Settings are the user's other choices, e.g. the colour theme.
	Settings Settings `json:"settings"`
}

// Settings are values of the panel's settings by their keys, e.g. {"theme": "dark"}. Keys a
// user never set follow the browser.
type Settings map[string]string

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
	p := Preferences{Dashboard: []Widget{}, Pinned: []Server{}, Settings: Settings{}}
	if err := s.readJSON(ctx, `SELECT widgets FROM dashboards WHERE user_id = ?`, userID, &p.Dashboard); err != nil {
		return p, err
	}
	if err := s.readJSON(ctx, `SELECT settings FROM user_settings WHERE user_id = ?`, userID, &p.Settings); err != nil {
		return p, err
	}
	// Values that only an older version knew are left out.
	maps.DeleteFunc(p.Settings, func(key, value string) bool { return !slices.Contains(settings[key], value) })
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

// ChangeSettings changes some of a user's settings: a value sets its key, null removes it so
// that it follows the browser again. The other keys keep their values, also those that another
// change sets at the same time.
func (s *Store) ChangeSettings(ctx context.Context, userID int64, change map[string]*string) error {
	if err := checkSettings(change); err != nil || len(change) == 0 {
		return err
	}
	data, err := json.Marshal(change)
	if err != nil {
		return err
	}
	// json_patch merges as RFC 7396 describes, in a single statement.
	_, err = s.db.ExecContext(ctx, `INSERT INTO user_settings (user_id, settings) VALUES (?1, json_patch('{}', ?2))
		ON CONFLICT (user_id) DO UPDATE SET settings = json_patch(settings, ?2)`, userID, string(data))
	return err
}

// readJSON decodes the JSON that query returns for a user into v, which stays as it is
// without a row.
func (s *Store) readJSON(ctx context.Context, query string, userID int64, v any) error {
	var data string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), v)
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

func checkSettings(change map[string]*string) error {
	for _, key := range slices.Sorted(maps.Keys(change)) {
		values, known := settings[key]
		switch {
		case !known:
			return httpapi.Errorf(http.StatusBadRequest, "The panel has no setting %q.", key)
		case change[key] != nil && !slices.Contains(values, *change[key]):
			return httpapi.Errorf(http.StatusBadRequest, "The setting %q is one of %s, or null.", key, strings.Join(values, ", "))
		}
	}
	return nil
}
