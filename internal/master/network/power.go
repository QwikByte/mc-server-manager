package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"unicode"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const maxBroadcast = 256

// Actions on all servers of a network.
const (
	Start   = "start"
	Stop    = "stop"
	Restart = "restart"
)

type serverCall func(ctx context.Context, c noryxv1.ServerServiceClient, id string) error

var (
	start = func(ctx context.Context, c noryxv1.ServerServiceClient, id string) error {
		_, err := c.StartServer(ctx, &noryxv1.StartServerRequest{Id: id})
		return err
	}
	stop = func(ctx context.Context, c noryxv1.ServerServiceClient, id string) error {
		_, err := c.StopServer(ctx, &noryxv1.StopServerRequest{Id: id})
		return err
	}
	restart = func(ctx context.Context, c noryxv1.ServerServiceClient, id string) error {
		_, err := c.RestartServer(ctx, &noryxv1.RestartServerRequest{Id: id})
		return err
	}
)

// Power starts, stops or restarts all servers of a network in the order that suits its
// players: the game servers start before the proxy, so that players find them, and the
// proxy stops first, so that players leave the network at once instead of server by server.
func (s *Service) Power(ctx context.Context, n Network, action string) error {
	ctx = context.WithoutCancel(ctx)
	proxy := []Backend{{Ref: n.Proxy, Name: "the proxy"}}
	switch action {
	case Start:
		return errors.Join(s.each(ctx, n.Backends, start), s.each(ctx, proxy, start))
	case Stop:
		return errors.Join(s.each(ctx, proxy, stop), s.each(ctx, n.Backends, stop))
	case Restart:
		return errors.Join(s.each(ctx, proxy, stop), s.each(ctx, n.Backends, restart), s.each(ctx, proxy, start))
	}
	return httpapi.Errorf(http.StatusBadRequest, "Choose start, stop or restart.")
}

// Broadcast sends a chat message to the players of all running game servers of a network.
func (s *Service) Broadcast(ctx context.Context, n Network, text string) error {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxBroadcast || strings.ContainsFunc(text, unicode.IsControl) {
		return httpapi.Errorf(http.StatusBadRequest, "Enter a message of up to %d characters.", maxBroadcast)
	}
	return s.each(ctx, n.Backends, func(ctx context.Context, c noryxv1.ServerServiceClient, id string) error {
		_, err := c.SendCommand(ctx, &noryxv1.SendCommandRequest{Id: id, Command: "say " + text})
		if status.Code(err) == codes.FailedPrecondition {
			return nil // a stopped server has no players
		}
		return err
	})
}

// each calls the agents of the given servers at the same time.
func (s *Service) each(ctx context.Context, servers []Backend, call serverCall) error {
	errs := make([]error, len(servers))
	var wg sync.WaitGroup
	for i, b := range servers {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, configureTimeout)
			defer cancel()
			conn, err := s.nodes.Conn(ctx, b.NodeID)
			if err == nil {
				err = call(ctx, noryxv1.NewServerServiceClient(conn), b.ServerID)
			}
			if err != nil {
				errs[i] = fmt.Errorf("%s: %s", b.Name, message(err))
			}
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return httpapi.Errorf(http.StatusBadGateway, "%s", strings.ReplaceAll(err.Error(), "\n", " · "))
	}
	return nil
}
