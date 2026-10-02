package app

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/local"
)

// local connects to the running agent through its Unix socket.
func (c *config) local(_ context.Context, fn func(grpc.ClientConnInterface) error) error {
	conn, err := grpc.NewClient("unix://"+c.socket(), grpc.WithTransportCredentials(local.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	return fn(conn)
}
