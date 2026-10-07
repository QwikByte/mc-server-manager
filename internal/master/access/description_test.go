package access

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// An API token gets its user's grants of its permissions and those they require, in the
// user's scope, and never what only administrators may do.
func TestOnly(t *testing.T) {
	lobby := Target{"n1", "lobby"}
	user := group(withRequired([]Permission{ServersRestart, ServersStop, FilesWrite}), lobby)
	token := user.Only([]Permission{ServersRestart, FilesRead, BackupsCreate, "unknown.permission"})
	for _, tc := range []struct {
		name string
		got  bool
		want bool
	}{
		{"restart the lobby", token.On(ServersRestart, "n1", "lobby"), true},
		{"see the lobby, which restarting requires", token.On(ServersView, "n1", "lobby"), true},
		{"read files", token.On(FilesRead, "n1", "lobby"), true},
		{"stop, which the user may but the token not", token.On(ServersStop, "n1", "lobby"), false},
		{"write files, which the user may but the token not", token.On(FilesWrite, "n1", "lobby"), false},
		{"restart elsewhere", token.Somewhere(ServersRestart, "n2"), false},
		{"back up, which the user may not", token.Somewhere(BackupsCreate, ""), false},
		{"administrator's token", Admin().Only([]Permission{ServersStart}).On(ServersStart, "n9", "x"), true},
		{"administrator's token stops", Admin().Only([]Permission{ServersStart}).On(ServersStop, "n9", "x"), false},
		{"administrator's token is no administrator", Admin().Only([]Permission{ServersStart}).admin, false},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %v", tc.name, tc.got)
		}
	}
	if !user.Covers(token) {
		t.Error("the token has more than its user")
	}
}

// The description lists every route with the permissions it needs, also of needs it doesn't know.
func TestDescription(t *testing.T) {
	mux := http.NewServeMux()
	m := NewMux(mux)
	h := func(http.ResponseWriter, *http.Request) {}
	custom := func(_ *http.Request, g Grants) (Permission, bool) { return LogsView, g.Somewhere(LogsView, "") }
	m.Handle("GET /api/servers", SignedIn, h)
	m.Handle("POST /api/nodes/{node}/servers/{id}/move", All(OnServer(ServersDelete), OnServer(FilesRead)), h)
	m.Handle("DELETE /api/nodes/{id}", OnNode(NodesDelete, "id"), h)
	m.Handle("POST /api/update/master", AdminsOnly, h)
	m.Handle("GET /api/logs", custom, h)
	m.Handle("GET /api/files/{path...}", Everywhere(FilesRead), h)
	(&Handler{}).Register(m)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil))
	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			Permissions []Permission `json:"x-permissions"`
			Parameters  []struct {
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil || rec.Code != http.StatusOK || doc.OpenAPI != "3.1.0" {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	for route, want := range map[[2]string][]Permission{
		{"/api/servers", "get"}:                          {},
		{"/api/nodes/{node}/servers/{id}/move", "post"}:  {ServersDelete, FilesRead},
		{"/api/nodes/{id}", "delete"}:                    {NodesDelete},
		{"/api/update/master", "post"}:                   {Administrators},
		{"/api/logs", "get"}:                             {LogsView},
		{"/api/files/{path}", "get"}:                     {FilesRead},
		{"/api/openapi.json", "get"}:                     {},
		{"/api/groups", "post"}:                          {GroupsManage},
		{"/api/users/{id}/setup-link", "post"}:           {UsersManage},
		{"/api/access/permissions", "get"}:               {},
		{"/api/nodes/{node}/servers/{id}/move", "patch"}: nil,
	} {
		op, ok := doc.Paths[route[0]][route[1]]
		if ok != (want != nil) || !slices.Equal(op.Permissions, want) {
			t.Errorf("%s %s: %v, want %v", route[1], route[0], op.Permissions, want)
		}
	}
	if params := doc.Paths["/api/nodes/{node}/servers/{id}/move"]["post"].Parameters; len(params) != 2 || params[0].Name != "node" || params[1].In != "path" {
		t.Errorf("parameters = %+v", params)
	}
}
