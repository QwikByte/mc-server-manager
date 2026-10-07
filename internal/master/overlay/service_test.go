package overlay

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

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
