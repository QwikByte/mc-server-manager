package overlay

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"google.golang.org/grpc"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/database"
	"github.com/QwikByte/noryx/internal/master/node"
)

type fakeNodes struct{}

func (fakeNodes) Get(_ context.Context, id string) (node.Node, error) {
	return node.Node{ID: id, Name: id, Address: "203.0.113.1:7443"}, nil
}

func (fakeNodes) Conn(context.Context, string) (grpc.ClientConnInterface, error) {
	return nil, errors.New("no agent")
}

// Changes that the database refuses fail, e.g. as the request ended while another change ran.
func TestDatabaseErrors(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewService(db, fakeNodes{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.SetEndpoint(ctx, "node", "203.0.113.1:51820"); !errors.Is(err, context.Canceled) {
		t.Errorf("set endpoint: %v", err)
	}
	if err := s.Leave(ctx, "node"); !errors.Is(err, context.Canceled) {
		t.Errorf("leave: %v", err)
	}
}

// A member gets the ports that another publishes for its address with its current key, and
// none that it would refuse, so that one member's report can't break another's configuration.
func TestPublishedFor(t *testing.T) {
	m := Member{Address: "10.213.0.1", PublicKey: "key"}
	client := func(address, key string) []*noryxv1.OverlayClient {
		return []*noryxv1.OverlayClient{{Address: "10.213.0.9", PublicKey: "other"}, {Address: address, PublicKey: key}}
	}
	report := &noryxv1.GetOverlayResponse{Published: []*noryxv1.OverlayPublished{
		{Port: 25570, ServerId: "survival", Clients: client("10.213.0.1", "key")},
		{Port: 25571, ServerId: "old key", Clients: client("10.213.0.1", "rotated")},
		{Port: 25572, ServerId: "another node", Clients: client("10.213.0.2", "key")},
		{Port: 0, ServerId: "no port", Clients: client("10.213.0.1", "key")},
		{Port: 65536, ServerId: "beyond", Clients: client("10.213.0.1", "key")},
		{Port: 3306, DatastoreId: "main", Clients: client("10.213.0.1", "key")},
	}}
	var got []uint32
	for _, p := range publishedFor(report, m) {
		got = append(got, p.GetPort())
	}
	if !slices.Equal(got, []uint32{25570, 3306}) {
		t.Errorf("ports %v", got)
	}
	if publishedFor(nil, m) != nil {
		t.Error("ports of a member that didn't answer")
	}
}
