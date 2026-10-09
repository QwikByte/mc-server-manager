// Package preference keeps each user's preferences of the panel: the layout of their
// overview, the servers they pinned, which warnings and errors pop up, and settings such as
// the colour theme. They follow the user into every browser.
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
	"time"
	_ "time/tzdata" // the time zone of the panel is an IANA time zone, which minimal systems don't have

	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	maxWidgets    = 32
	maxColumns    = 3
	maxPinned     = 20
	maxChosen     = 100
	maxCategories = 32
)

var (
	widgetPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	// idPattern matches the IDs of nodes and servers, which are random base32 in lowercase.
	idPattern = regexp.MustCompile(`^[a-z2-7]{26}$`)
	// categoryPattern matches the categories of the log, as the notifications do.
	categoryPattern = regexp.MustCompile(`^[a-z][a-z-]{0,31}$`)
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
	// The console, the terminal and the editor: the size of their text, and whether they
	// follow the colour theme or stay dark.
	"codeSize":  {"small", "medium", "large"},
	"codeTheme": {"dark", "panel"},
	// Whether the console and the terminal wrap long lines, whether the console shows the time
	// of each line, how many lines it keeps and whether it only shows warnings and errors.
	"consoleWrap":   {"wrap", "scroll"},
	"terminalWrap":  {"scroll", "wrap"},
	"consoleTimes":  {"hide", "show"},
	"consoleLines":  {"2000", "5000", "10000"},
	"consoleFilter": {"all", "problems"},
	// Whether the editor wraps long lines, how it indents and which keys it follows.
	"editorWrap":   {"off", "on"},
	"editorIndent": {"2", "4", "tab"},
	"editorKeys":   {"standard", "vim"},
	// Whether times show how long ago they were or the date and time, the first day of the
	// week, and the time zone of times, an IANA time zone (see valid).
	"times":     {"relative", "absolute"},
	"weekStart": {"monday", "sunday"},
	"timeZone":  nil,
	// The separator of the cells of CSV files, and whether they start with a byte order mark.
	"csvSeparator": {"comma", "semicolon"},
	"csvBom":       {"off", "on"},
	// What the user chose last where the address doesn't say: the tab and sort of the players,
	// the source, sort and software of the plugin search, whose software is all or one with
	// plugins or with mods, the sort of a server's plugins, and the range and view of usage.
	"playerTab":        {"online", "seen", "banned", "whitelisted", "operators"},
	"playerSort":       {"name", "server", "network", "seen", "playtime", "servers"},
	"playerOrder":      {"asc", "desc"},
	"pluginSource":     {"modrinth", "hangar"},
	"pluginSort":       {"relevance", "downloads", "follows", "newest", "updated"},
	"pluginType":       {"all", "paper", "purpur", "folia", "leaf", "velocity", "bungeecord", "waterfall"},
	"modType":          {"all", "fabric", "quilt", "forge", "neoforge"},
	"serverPluginSort": {"name", "size"},
	"usageRange":       {"day", "week"},
	"usageView":        {"charts", "table"},
	// The least level the log shows where its address names none; without, it shows all.
	"logLevel": {"info", "warn", "error"},
	// Where the panel opens, the address of a page of its sidebar, and the tab a server opens on, both only if the
	// user may see them; whether pages fill wide screens, move less whatever the operating system asks, and follow
	// shortcuts of single keys.
	"startPage": {"/", "/servers", "/networks", "/players", "/nodes", "/templates", "/filesets", "/plugins", "/backups", "/policies",
		"/workflows", "/agenda", "/logs", "/settings"},
	"serverTab": {"console", "usage", "files", "settings"},
	"width":     {"limited", "full"},
	"motion":    {"system", "less"},
	"shortcuts": {"on", "off"},
}

// valid tells whether a setting takes a value.
func valid(key, value string) bool {
	if key == "timeZone" {
		// LoadLocation also takes "" and "Local", which mean the master's own time zone. IANA
		// names are short, so longer ones aren't looked up at all.
		if len(value) > 64 || value == "" || value == "Local" {
			return false
		}
		_, err := time.LoadLocation(value)
		return err == nil
	}
	return slices.Contains(settings[key], value)
}

// Preferences are what a user chose for the panel.
type Preferences struct {
	// Dashboard is the order of the widgets of the overview; empty shows the panel's default layout.
	Dashboard []Widget `json:"dashboard"`
	// Pinned are the servers the user pinned, in their order.
	Pinned []Server `json:"pinned"`
	// Alerts choose which new warnings and errors pop up.
	Alerts Alerts `json:"alerts"`
	// Settings are the user's other choices, e.g. the colour theme.
	Settings Settings `json:"settings"`
}

// Alerts choose which new warnings and errors of the log the panel shows the user as they
// come, as toasts and on the desktop. The bell lists all of them either way.
type Alerts struct {
	// Level is the least level that pops up: "warn" or "error".
	Level string `json:"level"`
	// Only lets only the entries of pinned servers pop up if Pinned, and those about the
	// chosen nodes, servers and categories, rather than all.
	Only       bool     `json:"only"`
	Pinned     bool     `json:"pinned"`
	Nodes      []string `json:"nodes"`
	Servers    []string `json:"servers"`
	Categories []string `json:"categories"`
	// QuietUntil keeps all of them from popping up until then, e.g. for an hour.
	QuietUntil *time.Time `json:"quietUntil,omitempty"`
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
	p := Preferences{Dashboard: []Widget{}, Pinned: []Server{}, Alerts: Alerts{Level: "warn"}, Settings: Settings{}}
	if err := s.readJSON(ctx, `SELECT widgets FROM dashboards WHERE user_id = ?`, userID, &p.Dashboard); err != nil {
		return p, err
	}
	if err := s.readJSON(ctx, `SELECT filter FROM alert_filters WHERE user_id = ?`, userID, &p.Alerts); err != nil {
		return p, err
	}
	p.Alerts.Nodes, p.Alerts.Servers, p.Alerts.Categories = list(p.Alerts.Nodes), list(p.Alerts.Servers), list(p.Alerts.Categories)
	if err := s.readJSON(ctx, `SELECT settings FROM user_settings WHERE user_id = ?`, userID, &p.Settings); err != nil {
		return p, err
	}
	// Values that only an older version knew are left out.
	maps.DeleteFunc(p.Settings, func(key, value string) bool { return !valid(key, value) })
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

// SetAlerts stores which new warnings and errors pop up for a user.
func (s *Store) SetAlerts(ctx context.Context, userID int64, alerts Alerts) error {
	if err := checkAlerts(&alerts); err != nil {
		return err
	}
	data, err := json.Marshal(alerts)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO alert_filters (user_id, filter) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET filter = excluded.filter`, userID, string(data))
	return err
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

// Forget unpins a deleted server for every user, and takes it out of the users' alerts.
func (s *Store) Forget(ctx context.Context, nodeID, serverID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM pinned_servers WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	if err != nil {
		return err
	}
	return s.changeAlerts(ctx, func(a *Alerts) bool {
		n := len(a.Servers)
		a.Servers = slices.DeleteFunc(a.Servers, func(id string) bool { return id == serverID })
		return len(a.Servers) != n
	})
}

// Prune takes nodes that were removed out of the users' alerts; their pins went with them.
func (s *Store) Prune(ctx context.Context) error {
	nodes, err := database.IDs(ctx, s.db, `SELECT id FROM nodes`)
	if err != nil {
		return err
	}
	return s.changeAlerts(ctx, func(a *Alerts) bool {
		n := len(a.Nodes)
		a.Nodes = slices.DeleteFunc(a.Nodes, func(id string) bool { return !nodes[id] })
		return len(a.Nodes) != n
	})
}

// changeAlerts changes the alerts of the users with change, which tells whether it changed them.
func (s *Store) changeAlerts(ctx context.Context, change func(*Alerts) bool) error {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, filter FROM alert_filters`)
	if err != nil {
		return err
	}
	type filter struct {
		user int64
		data string
	}
	changed := map[filter]Alerts{}
	for rows.Next() {
		var f filter
		var a Alerts
		if err := rows.Scan(&f.user, &f.data); err != nil {
			rows.Close()
			return err
		}
		if json.Unmarshal([]byte(f.data), &a) == nil && change(&a) {
			changed[f] = a
		}
	}
	err = errors.Join(rows.Err(), rows.Close())
	for f, a := range changed {
		data, merr := json.Marshal(a)
		if merr == nil {
			// Alerts that the user changed in the meantime keep what the user chose.
			_, merr = s.db.ExecContext(ctx, `UPDATE alert_filters SET filter = ? WHERE user_id = ? AND CAST(filter AS TEXT) = ?`, string(data), f.user, f.data)
		}
		err = errors.Join(err, merr)
	}
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

// checkAlerts checks alerts, and sorts their lists without repeats.
func checkAlerts(a *Alerts) error {
	for _, l := range []*[]string{&a.Nodes, &a.Servers, &a.Categories} {
		*l = list(*l)
		slices.Sort(*l)
		*l = slices.Compact(*l)
	}
	switch {
	case a.Level != "warn" && a.Level != "error":
		return httpapi.Errorf(http.StatusBadRequest, "Choose the level warn or error.")
	case len(a.Nodes) > maxChosen || len(a.Servers) > maxChosen:
		return httpapi.Errorf(http.StatusBadRequest, "Choose up to %d nodes and %d servers.", maxChosen, maxChosen)
	case slices.ContainsFunc(a.Nodes, invalidID) || slices.ContainsFunc(a.Servers, invalidID):
		return httpapi.Errorf(http.StatusBadRequest, "Nodes and servers are chosen by their IDs.")
	case len(a.Categories) > maxCategories || slices.ContainsFunc(a.Categories, func(c string) bool { return !categoryPattern.MatchString(c) }):
		return httpapi.Errorf(http.StatusBadRequest, "Choose up to %d categories of the log.", maxCategories)
	}
	return nil
}

func invalidID(id string) bool { return !idPattern.MatchString(id) }

// list returns l, or an empty list for nil, so that JSON has a list.
func list(l []string) []string {
	if l == nil {
		return []string{}
	}
	return l
}

func checkSettings(change map[string]*string) error {
	for _, key := range slices.Sorted(maps.Keys(change)) {
		values, known := settings[key]
		switch {
		case !known:
			return httpapi.Errorf(http.StatusBadRequest, "The panel has no setting %q.", key)
		case change[key] == nil || valid(key, *change[key]):
		case key == "timeZone":
			return httpapi.Errorf(http.StatusBadRequest, "The setting %q is an IANA time zone, e.g. Europe/Berlin, or null.", key)
		default:
			return httpapi.Errorf(http.StatusBadRequest, "The setting %q is one of %s, or null.", key, strings.Join(values, ", "))
		}
	}
	return nil
}
