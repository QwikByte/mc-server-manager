package preference

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

func open(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func id() string { return strings.ToLower(rand.Text()) }

func exec(t *testing.T, db *sql.DB, query string, args ...any) sql.Result {
	t.Helper()
	res, err := db.Exec(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func addUser(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	userID, err := exec(t, db, `INSERT INTO users (username, password_hash, created_at) VALUES (?, '', 0)`, name).LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return userID
}

func addNode(t *testing.T, db *sql.DB) string {
	t.Helper()
	nodeID := id()
	exec(t, db, `INSERT INTO nodes (id, name, address, created_at) VALUES (?, ?, 'host:7443', 0)`, nodeID, nodeID)
	return nodeID
}

func TestValidation(t *testing.T) {
	db := open(t)
	s, ctx, user, node := NewStore(db), t.Context(), addUser(t, db, "alice"), addNode(t, db)
	widgets := func(n int) []Widget {
		w := make([]Widget, n)
		for i := range w {
			w[i] = Widget{ID: fmt.Sprint("w", i), Columns: 1}
		}
		return w
	}
	servers := func(n int) []Server {
		srv := make([]Server, n)
		for i := range srv {
			srv[i] = Server{NodeID: node, ServerID: id()}
		}
		return srv
	}
	badRequest := func(err error) bool {
		var apiErr *httpapi.Error
		return errors.As(err, &apiErr) && apiErr.Status == http.StatusBadRequest
	}

	for name, w := range map[string][]Widget{
		"no id":         {{Columns: 1}},
		"capitals":      {{ID: "Servers", Columns: 1}},
		"leading digit": {{ID: "1servers", Columns: 1}},
		"path":          {{ID: "../servers", Columns: 1}},
		"long id":       {{ID: "s" + strings.Repeat("x", 32), Columns: 1}},
		"twice":         {{ID: "servers", Columns: 1}, {ID: "servers", Columns: 2}},
		"no columns":    {{ID: "servers"}},
		"four columns":  {{ID: "servers", Columns: 4}},
		"too many":      widgets(maxWidgets + 1),
	} {
		if err := s.SetDashboard(ctx, user, w); !badRequest(err) {
			t.Errorf("%s: SetDashboard = %v, want a bad request", name, err)
		}
	}
	srv := id()
	for name, pins := range map[string][]Server{
		"no node":    {{ServerID: srv}},
		"no server":  {{NodeID: node}},
		"capitals":   {{NodeID: strings.ToUpper(node), ServerID: srv}},
		"short":      {{NodeID: node, ServerID: srv[1:]}},
		"path":       {{NodeID: node, ServerID: "../" + srv[3:]}},
		"bad letter": {{NodeID: node, ServerID: "0" + srv[1:]}},
		"twice":      {{NodeID: node, ServerID: srv}, {NodeID: node, ServerID: srv}},
		"too many":   servers(maxPinned + 1),
	} {
		if err := s.SetPinned(ctx, user, pins); !badRequest(err) {
			t.Errorf("%s: SetPinned = %v, want a bad request", name, err)
		}
	}
	if p, err := s.Get(ctx, user); err != nil || len(p.Dashboard) != 0 || len(p.Pinned) != 0 {
		t.Fatalf("after invalid changes, Get = %+v, %v", p, err)
	}

	// The limits themselves are fine.
	most := widgets(maxWidgets)
	most[0] = Widget{ID: "s" + strings.Repeat("x", 31), Columns: maxColumns, Hidden: true}
	if err := s.SetDashboard(ctx, user, most); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPinned(ctx, user, servers(maxPinned)); err != nil {
		t.Fatal(err)
	}
}

func TestStore(t *testing.T) {
	db := open(t)
	s, ctx := NewStore(db), t.Context()
	alice, bob := addUser(t, db, "alice"), addUser(t, db, "bob")
	n1, n2 := addNode(t, db), addNode(t, db)
	lobby, game := id(), id()
	check := func(user int64, want Preferences) {
		t.Helper()
		got, err := s.Get(ctx, user)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("Get(%d) = %+v, %v, want %+v", user, got, err, want)
		}
	}
	none := Preferences{Dashboard: []Widget{}, Pinned: []Server{}}
	// Users start with the default layout and no pins, as empty lists rather than nil.
	check(alice, none)

	dashboard := []Widget{{ID: "servers", Columns: 2}, {ID: "nodes", Columns: 1, Hidden: true}, {ID: "usage", Columns: 3}}
	pinned := []Server{{n2, game}, {n1, lobby}, {n1, game}}
	if err := s.SetDashboard(ctx, alice, dashboard); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPinned(ctx, alice, pinned); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPinned(ctx, bob, []Server{{n1, lobby}}); err != nil {
		t.Fatal(err)
	}
	check(alice, Preferences{Dashboard: dashboard, Pinned: pinned})
	check(bob, Preferences{Dashboard: []Widget{}, Pinned: []Server{{n1, lobby}}})

	// New lists replace the old ones in their order; an empty layout brings back the default.
	// Servers of nodes that were removed in the meantime aren't pinned.
	dashboard = []Widget{dashboard[2], dashboard[0]}
	if err := s.SetDashboard(ctx, alice, dashboard); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPinned(ctx, alice, []Server{{n1, game}, {id(), lobby}, {n1, lobby}, {n2, game}}); err != nil {
		t.Fatal(err)
	}
	check(alice, Preferences{Dashboard: dashboard, Pinned: []Server{{n1, game}, {n1, lobby}, {n2, game}}})
	if err := s.SetDashboard(ctx, alice, nil); err != nil {
		t.Fatal(err)
	}
	check(alice, Preferences{Dashboard: []Widget{}, Pinned: []Server{{n1, game}, {n1, lobby}, {n2, game}}})

	// A server that moves stays pinned, once for those who pinned it at both places; a deleted
	// one is unpinned for everyone, as are the servers of a removed node.
	if err := s.Move(ctx, game, n1, n2); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(ctx, lobby, n1, n2); err != nil {
		t.Fatal(err)
	}
	check(alice, Preferences{Dashboard: []Widget{}, Pinned: []Server{{n2, lobby}, {n2, game}}})
	check(bob, Preferences{Dashboard: []Widget{}, Pinned: []Server{{n2, lobby}}})
	if err := s.Forget(ctx, n2, lobby); err != nil {
		t.Fatal(err)
	}
	check(alice, Preferences{Dashboard: []Widget{}, Pinned: []Server{{n2, game}}})
	check(bob, none)
	exec(t, db, `DELETE FROM nodes WHERE id = ?`, n2)
	check(alice, none)

	// The preferences go with the user.
	if err := s.SetDashboard(ctx, alice, dashboard); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPinned(ctx, alice, []Server{{n1, lobby}}); err != nil {
		t.Fatal(err)
	}
	exec(t, db, `DELETE FROM users WHERE id = ?`, alice)
	var left int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM dashboards) + (SELECT COUNT(*) FROM pinned_servers)`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d preferences left of a deleted user, %v", left, err)
	}
}
