package e2e

import (
	"net/http"
	"testing"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
)

func TestUpdateImage(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	lobby := m.createServer(t, a, "Lobby", noryxv1.ServerType_SERVER_TYPE_PAPER, 25565)
	servers := "/api/nodes/" + a.node.ID + "/servers/"

	var res struct{ Updated bool }
	api.do("POST", servers+lobby.ServerID+"/update-image", nil, http.StatusOK, &res)
	if !res.Updated {
		t.Error("the newer image wasn't reported")
	}
	api.do("POST", servers+"aaaaaaaaaaaaaaaaaaaaaaaaaa/update-image", nil, http.StatusNotFound, nil)
}
