package e2e

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	agentenroll "github.com/QwikByte/noryx/internal/agent/enroll"
	"github.com/QwikByte/noryx/internal/master/node"
	"github.com/QwikByte/noryx/internal/pki"
)

// Errors that mean a node can't be used name it, instead of passing on those of Docker or gRPC.
func TestNodeErrors(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + a.node.ID
	type nodeView struct {
		Status               string
		CertificateExpiresAt *time.Time
	}
	var online nodeView
	api.do("GET", path, nil, http.StatusOK, &online)
	if online.Status != "online" || online.CertificateExpiresAt == nil {
		t.Fatalf("online node = %+v", online)
	}

	a.runtime.mu.Lock()
	a.runtime.down = true
	a.runtime.mu.Unlock()
	if body := api.do("GET", path+"/servers", nil, http.StatusBadGateway, nil); !strings.Contains(body, "Docker isn't running on node-1, or its agent can't connect to it.") {
		t.Errorf("while Docker is down: %s", body)
	}

	gone := listen(t)
	gone.Close()
	api.do("PUT", path, map[string]any{"name": "node-1", "address": gone.Addr().String(), "defaultStorage": "default"}, http.StatusOK, nil)
	if body := api.do("GET", path+"/servers", nil, http.StatusBadGateway, nil); !strings.Contains(body, "node-1 can't be reached.") {
		t.Errorf("while the agent can't be reached: %s", body)
	}

	// The panel still learns when the certificate of the offline node expires.
	var offline nodeView
	api.do("GET", path, nil, http.StatusOK, &offline)
	if offline.Status != "offline" || offline.CertificateExpiresAt == nil || !offline.CertificateExpiresAt.Equal(*online.CertificateExpiresAt) {
		t.Fatalf("offline node = %+v, online it was %+v", offline, online)
	}

	// So does a master that restarted while the node is offline.
	m.nodes = node.NewService(m.db, m.ca, m.cert, m.settings)
	t.Cleanup(m.nodes.Close)
	var restarted nodeView
	apiClient{t: t, url: m.panel(t).URL}.do("GET", path, nil, http.StatusOK, &restarted)
	if restarted.Status != "offline" || restarted.CertificateExpiresAt == nil || !restarted.CertificateExpiresAt.Equal(*online.CertificateExpiresAt) {
		t.Fatalf("offline node after a restart = %+v, online it was %+v", restarted, online)
	}
}

func TestNodeSettings(t *testing.T) {
	m := startMaster(t)
	a := m.startAgent(t, "node-1")
	m.startAgent(t, "node-2")
	api := apiClient{t: t, url: m.panel(t).URL}
	path := "/api/nodes/" + a.node.ID
	settings := func(change func(map[string]any)) map[string]any {
		s := map[string]any{"name": "Frankfurt 1", "address": a.node.Address, "defaultStorage": "default", "portMin": 25565, "portMax": 25570, "memoryReserveMb": 12288}
		change(s)
		return s
	}

	var got struct {
		Name            string `json:"name"`
		PortMin         *int   `json:"portMin"`
		MemoryReserveMB *int   `json:"memoryReserveMb"`
		Status          string `json:"status"`
	}
	api.do("PUT", path, settings(func(map[string]any) {}), http.StatusOK, &got)
	if got.Name != "Frankfurt 1" || got.PortMin == nil || *got.PortMin != 25565 || *got.MemoryReserveMB != 12288 || got.Status != "online" {
		t.Fatalf("updated node = %+v", got)
	}
	api.do("PUT", path, settings(func(s map[string]any) { s["name"] = "node-2" }), http.StatusConflict, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["portMin"] = 30000 }), http.StatusBadRequest, nil)
	api.do("PUT", path, settings(func(s map[string]any) { s["portMax"] = nil }), http.StatusBadRequest, nil)

	// The node has 16 GB, of which 12 GB are kept free, and allows ports 25565 to 25570. A
	// server with 1 GB counts with the 1.5 GB of its container.
	create := func(name string, memory, port, status int) string {
		var srv struct{ ID string }
		api.do("POST", path+"/servers", map[string]any{"name": name, "type": "paper", "memoryMb": memory, "port": port, "acceptEula": true}, status, &srv)
		return srv.ID
	}
	create("Outside", 1024, 25600, http.StatusBadRequest)
	lobby := create("Lobby", 1024, 25565, http.StatusCreated)
	create("Big", 2048, 25566, http.StatusConflict)
	create("Survival", 1024, 25566, http.StatusCreated)
	update := map[string]any{"name": "Lobby", "version": "LATEST", "memoryMb": 2048, "port": 25565, "restartPolicy": "always"}
	api.do("PUT", path+"/servers/"+lobby, update, http.StatusConflict, nil)

	// Without a reserve, memory isn't limited.
	api.do("PUT", path, settings(func(s map[string]any) { s["memoryReserveMb"] = nil }), http.StatusOK, nil)
	api.do("PUT", path+"/servers/"+lobby, update, http.StatusOK, nil)
}

// The enrollment endpoint checks the join token before it signs a certificate, and slows
// down clients that keep trying.
func TestEnrollmentChecks(t *testing.T) {
	m := startMaster(t)
	n, token, err := m.nodes.Create(t.Context(), "node-1", "127.0.0.1:7443")
	check(t, err)
	guesser := peer.NewContext(t.Context(), &peer.Peer{Addr: &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 1234}})
	enroll := func(secret string, csr []byte) codes.Code {
		_, err := m.nodes.Enroll(guesser, &noryxv1.EnrollRequest{NodeId: n.ID, Secret: secret, CsrDer: csr})
		return status.Code(err)
	}

	// An invalid request doesn't use up the token, a wrong token gets nothing signed.
	if code := enroll(token.Secret, []byte("not a CSR")); code != codes.InvalidArgument {
		t.Fatalf("invalid CSR: %v", code)
	}
	csr, err := pki.NewCSR(pki.NewKey())
	check(t, err)
	for range 4 {
		if code := enroll("guessed", csr); code != codes.PermissionDenied {
			t.Fatalf("wrong token: %v", code)
		}
	}
	if code := enroll(token.Secret, csr); code != codes.ResourceExhausted {
		t.Fatalf("client over its budget: %v", code)
	}

	// Other clients still enroll with the token.
	check(t, agentenroll.Run(t.Context(), token.String(), t.TempDir()))
}
