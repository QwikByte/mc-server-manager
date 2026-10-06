package e2e

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/preference"
)

// Each user keeps the layout of their overview and the servers they pinned, which every
// browser of the user gets. A deleted server is unpinned.
func TestPreferences(t *testing.T) {
	m := startMaster(t)
	svc := m.services(t)
	srv := httptest.NewTLSServer(masterapp.Handler(svc))
	t.Cleanup(srv.Close)
	a := m.startAgent(t, "node-1")
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, a, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	admin, err := svc.Users.CreateUser(t.Context(), "admin", "the-admins-password")
	check(t, err)
	check(t, svc.Access.MakeAdmin(t.Context(), admin.ID))
	_, err = svc.Users.CreateUser(t.Context(), "other", "the-others-password")
	check(t, err)
	login := func(username, password string) apiClient {
		b := browser(t, srv)
		b.do("POST", "/api/auth/login", map[string]string{"username": username, "password": password}, http.StatusOK, nil)
		return b
	}
	browser(t, srv).do("GET", "/api/preferences", nil, http.StatusUnauthorized, nil)

	first := login("admin", "the-admins-password")
	if body := first.do("GET", "/api/preferences", nil, http.StatusOK, nil); strings.TrimSpace(body) != `{"dashboard":[],"pinned":[]}` {
		t.Fatalf("preferences of a new user = %s", body)
	}
	want := preference.Preferences{
		Dashboard: []preference.Widget{{ID: "servers", Columns: 2}, {ID: "nodes", Columns: 1, Hidden: true}},
		Pinned:    []preference.Server{{NodeID: game.NodeID, ServerID: game.ServerID}, {NodeID: lobby.NodeID, ServerID: lobby.ServerID}},
	}
	var got preference.Preferences
	first.do("PUT", "/api/preferences/dashboard", map[string]any{"widgets": want.Dashboard}, http.StatusOK, nil)
	first.do("PUT", "/api/preferences/pinned", map[string]any{"servers": want.Pinned}, http.StatusOK, &got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preferences = %+v, want %+v", got, want)
	}
	for path, body := range map[string]any{
		"dashboard": map[string]any{"widgets": []preference.Widget{{ID: "servers", Columns: 4}}},
		"pinned":    map[string]any{"servers": append(want.Pinned, want.Pinned[0])},
	} {
		first.do("PUT", "/api/preferences/"+path, body, http.StatusBadRequest, nil)
	}
	first.do("PUT", "/api/preferences/pinned", map[string]any{"servers": []map[string]string{{"nodeId": "../" + game.NodeID, "serverId": game.ServerID}}}, http.StatusBadRequest, nil)
	first.do("PUT", "/api/preferences/dashboard", map[string]any{"widgets": want.Dashboard, "userId": 2}, http.StatusBadRequest, nil)

	// Another browser of the user gets them, another user doesn't.
	second := login("admin", "the-admins-password")
	second.do("GET", "/api/preferences", nil, http.StatusOK, &got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("preferences in another browser = %+v, want %+v", got, want)
	}
	login("other", "the-others-password").do("GET", "/api/preferences", nil, http.StatusOK, &got)
	if len(got.Dashboard) != 0 || len(got.Pinned) != 0 {
		t.Fatalf("preferences of another user = %+v", got)
	}

	second.do("DELETE", "/api/nodes/"+game.NodeID+"/servers/"+game.ServerID, nil, http.StatusNoContent, nil)
	first.do("GET", "/api/preferences", nil, http.StatusOK, &got)
	if !reflect.DeepEqual(got.Pinned, want.Pinned[1:]) {
		t.Fatalf("pinned after deleting a server = %+v", got.Pinned)
	}
}
