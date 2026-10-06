package e2e

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	agentoverlay "github.com/QwikByte/noryx/internal/agent/overlay"
	"github.com/QwikByte/noryx/internal/master/datastore"
	"github.com/QwikByte/noryx/internal/master/fileset"
	"github.com/QwikByte/noryx/internal/master/network"
	"github.com/QwikByte/noryx/internal/master/schedule"
)

// A network gets a datastore on one node, which its servers there reach by name and those of
// another node over the private network, with databases whose fields file sets fill in. The
// API never shows the passwords.
func TestDatastores(t *testing.T) {
	m := startMaster(t)
	a1, a2, a3 := m.startAgent(t, "node-1"), m.startAgent(t, "node-2"), m.startAgent(t, "node-3")
	proxy := m.createServer(t, a1, "Proxy", noryxv1.ServerType_SERVER_TYPE_VELOCITY, 25577)
	lobby := m.createServer(t, a1, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	game := m.createServer(t, a2, "Game", noryxv1.ServerType_SERVER_TYPE_PAPER, 25566)
	api := apiClient{t: t, url: m.panel(t).URL}
	for i, a := range []agent{a1, a2} {
		check(t, agentoverlay.Allow(a.dir))
		api.do("POST", "/api/nodes/"+a.node.ID+"/overlay", map[string]string{"endpoint": fmt.Sprintf("203.0.113.%d:51820", i+1)}, http.StatusOK, nil)
	}
	var n network.Network
	api.do("POST", "/api/networks", map[string]any{"name": "Main", "proxy": proxy, "servers": []network.Ref{lobby, game}}, http.StatusCreated, &n)
	base := "/api/networks/" + n.ID + "/datastores"

	for _, bad := range []map[string]any{
		{"name": "Main DB", "nodeId": a1.node.ID, "engine": "mariadb", "memoryMb": 512},
		{"name": "main", "nodeId": a1.node.ID, "engine": "mysql", "memoryMb": 512},
		{"name": "main", "nodeId": a1.node.ID, "engine": "mariadb", "memoryMb": 64},
		{"name": "main", "nodeId": a1.node.ID, "engine": "mariadb", "version": "10.6", "memoryMb": 512},
	} {
		api.do("POST", base, bad, http.StatusBadRequest, nil)
	}
	var ds datastore.View
	api.do("POST", base, map[string]any{"name": "main", "nodeId": a1.node.ID, "engine": "mariadb", "version": "11.8", "memoryMb": 512}, http.StatusCreated, &ds)
	if ds.State != "running" || ds.Version != "11.8" || !slices.Equal(ds.Versions, []string{"11.8", "12.3"}) || ds.NodeName != "node-1" {
		t.Fatalf("datastore = %+v", ds)
	}
	api.do("POST", base, map[string]any{"name": "main", "nodeId": a2.node.ID, "engine": "postgres", "memoryMb": 512}, http.StatusConflict, nil)

	// The servers of node-1 join it, the proxy too; node-2 reaches its port in the private network.
	for _, ref := range []network.Ref{proxy, lobby} {
		if got := a1.runtime.network(ref.ServerID).Datastores; !slices.Equal(got, []string{ds.ID}) {
			t.Fatalf("%s joined %v", ref.ServerID, got)
		}
	}
	if got := a2.runtime.network(game.ServerID).Datastores; len(got) > 0 {
		t.Fatalf("game joined %v on another node", got)
	}
	published := a1.kernel.applied().Clients["db-"+ds.ID]
	list, _ := a1.runtime.ListDatastores(t.Context())
	if published.Address.String() != "10.213.0.2" || published.Port == 0 || list[0].Port != uint32(published.Port) || list[0].Overlay != "10.213.0.1" {
		t.Fatalf("published %+v, datastore %+v", published, list[0])
	}

	dbs := "/api/datastores/" + ds.ID + "/databases"
	api.do("POST", dbs, map[string]string{"name": "luckperms"}, http.StatusCreated, nil)
	api.do("POST", dbs, map[string]string{"name": "luckperms"}, http.StatusCreated, nil) // again is fine
	api.do("POST", dbs, map[string]string{"name": "mysql"}, http.StatusBadRequest, nil)
	_, password := a1.runtime.Content(ds.ID, "luckperms")
	if !noryxv1.DatabasePassword.MatchString(password) {
		t.Fatalf("password %q", password)
	}

	// A file set fills in the fields of the database for each server.
	config := "plugins/LuckPerms/config.yml"
	in := fileset.Input{
		Name: "LuckPerms",
		Files: []fileset.File{{Path: config, Content: "address: {{datastore:main.luckperms.host}}:{{datastore:main.luckperms.port}}\n" +
			"database: {{datastore:main.luckperms.database}}\nusername: {{datastore:main.luckperms.user}}\npassword: {{datastore:main.luckperms.password}}\n"}},
		Targets: []fileset.Target{{Kind: fileset.KindNetwork, Value: n.ID, Role: fileset.RoleServers}},
	}
	var set fileset.Set
	api.do("POST", "/api/filesets", in, http.StatusCreated, &set)
	api.do("POST", "/api/filesets/"+set.ID+"/apply", map[string]any{"version": set.Version}, http.StatusOK, nil)
	read := func(a agent, ref network.Ref) string {
		data, err := os.ReadFile(filepath.Join(a.runtime.dir, ref.ServerID, filepath.FromSlash(config)))
		check(t, err)
		return string(data)
	}
	want := func(host string) string {
		return "address: " + host + "\ndatabase: luckperms\nusername: luckperms\npassword: " + password + "\n"
	}
	if got := read(a1, lobby); got != want("noryx-db-"+ds.ID+":3306") {
		t.Errorf("lobby has %q", got)
	}
	if got := read(a2, game); got != want("10.213.0.1:"+strconv.Itoa(int(published.Port))) {
		t.Errorf("game has %q", got)
	}
	body := api.do("GET", base, nil, http.StatusOK, nil)
	var listed struct {
		Datastores []datastore.View `json:"datastores"`
		Uses       []fileset.Use    `json:"uses"`
	}
	api.do("GET", base, nil, http.StatusOK, &listed)
	if strings.Contains(body, password) || len(listed.Uses) != 2 || listed.Uses[0].Database != "main.luckperms" || len(listed.Datastores[0].Databases) != 1 {
		t.Fatalf("listed %s", body)
	}

	// A new password reaches the servers that use the database.
	api.do("POST", dbs+"/luckperms/rotate", nil, http.StatusOK, nil)
	_, rotated := a1.runtime.Content(ds.ID, "luckperms")
	if rotated == password || !strings.Contains(read(a1, lobby), rotated) || !strings.Contains(read(a2, game), rotated) {
		t.Fatalf("after rotating, lobby has %q", read(a1, lobby))
	}

	// Dumps by hand and by a backup job, restored and downloaded.
	backups := "/api/datastores/" + ds.ID + "/backups"
	check(t, a1.runtime.Write(ds.ID, "luckperms", "groups v1"))
	var dump datastore.Dump
	api.do("POST", backups, map[string]any{"label": "before"}, http.StatusCreated, &dump)
	if !slices.Equal(dump.Databases, []string{"luckperms"}) || dump.Label != "before" {
		t.Fatalf("dump = %+v", dump)
	}
	check(t, a1.runtime.Write(ds.ID, "luckperms", "groups v2"))
	check(t, a1.runtime.setState(lobby.ServerID, noryxv1.ServerState_SERVER_STATE_RUNNING))
	api.do("POST", backups+"/"+dump.ID+"/restore", map[string]any{}, http.StatusNoContent, nil)
	if content, pw := a1.runtime.Content(ds.ID, "luckperms"); content != "groups v1" || pw != rotated {
		t.Fatalf("restored %q with password %q", content, pw)
	}
	if got := serverState(t, api, lobby); got != "running" {
		t.Errorf("lobby is %s after the restore", got)
	}
	archive := api.do("GET", backups+"/"+dump.ID+"/download", nil, http.StatusOK, nil)
	zr, err := zip.NewReader(strings.NewReader(archive), int64(len(archive)))
	check(t, err)
	f, err := zr.Open("luckperms.sql")
	check(t, err)
	if data, _ := io.ReadAll(f); string(data) != "groups v1" {
		t.Errorf("the download has %q", data)
	}
	job := map[string]any{
		"name": "Databases", "enabled": true, "targets": []map[string]string{},
		"schedule": map[string]any{"days": []int{}, "times": []string{"03:00"}, "timeZone": "Europe/Berlin"},
		"settings": map[string]any{"selection": map[string]any{}, "datastores": []string{ds.ID}, "keep": 1},
	}
	var task schedule.Task
	api.do("POST", "/api/backup-jobs", job, http.StatusCreated, &task)
	run(t, api, "/api/backup-jobs/"+task.ID)
	var dumps []datastore.Dump
	api.do("GET", backups, nil, http.StatusOK, &dumps)
	if len(dumps) != 2 || dumps[0].JobID != task.ID || dumps[0].Label != "Databases" {
		t.Fatalf("dumps = %+v", dumps)
	}

	// An upgrade keeps the data of the previous version until it is removed.
	api.do("PATCH", "/api/datastores/"+ds.ID, map[string]any{"version": "12.3"}, http.StatusOK, &ds)
	if ds.Version != "12.3" || ds.Previous != "11.8" {
		t.Fatalf("after the upgrade: %+v", ds)
	}
	var changed datastore.View
	api.do("PATCH", "/api/datastores/"+ds.ID, map[string]any{"removePrevious": true, "memoryMb": 1024}, http.StatusOK, &changed)
	if changed.Previous != "" || changed.MemoryMB != 1024 {
		t.Fatalf("after removing the previous version: %+v", changed)
	}

	// A move to a node that can't reach the datastore needs a confirmation, and the network
	// stays while it has datastores.
	body = api.do("POST", "/api/nodes/"+game.NodeID+"/servers/"+game.ServerID+"/move", map[string]any{"node": a3.node.ID}, http.StatusConflict, nil)
	if !strings.Contains(body, network.ErrUnreachable) {
		t.Errorf("move: %s", body)
	}
	api.do("DELETE", "/api/networks/"+n.ID, nil, http.StatusConflict, nil)

	// Deleting a datastore removes it with its dumps and closes its port.
	api.do("DELETE", "/api/datastores/"+ds.ID, nil, http.StatusNoContent, nil)
	if list, _ := a1.runtime.ListDatastores(t.Context()); len(list) > 0 {
		t.Fatalf("datastores left: %+v", list)
	}
	if _, ok := a1.kernel.applied().Clients["db-"+ds.ID]; ok {
		t.Error("the port stayed open")
	}
	api.do("GET", backups, nil, http.StatusNotFound, nil)
	api.do("DELETE", "/api/networks/"+n.ID, nil, http.StatusNoContent, nil)
}
