package player

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// olderAgent knows only ChangePlayer, and refuses invalid names.
type olderAgent struct {
	noryxv1.PlayerServiceClient
	changed []string
}

func (a *olderAgent) ChangePlayers(context.Context, *noryxv1.ChangePlayersRequest, ...grpc.CallOption) (*noryxv1.ChangePlayersResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unknown method")
}

func (a *olderAgent) ChangePlayer(_ context.Context, req *noryxv1.ChangePlayerRequest, _ ...grpc.CallOption) (*noryxv1.ChangePlayerResponse, error) {
	if req.GetChange().GetName() == "Nobody" {
		return nil, status.Error(codes.FailedPrecondition, "The server doesn't run.")
	}
	a.changed = append(a.changed, req.GetChange().GetName())
	return &noryxv1.ChangePlayerResponse{Output: "Banned " + req.GetChange().GetName()}, nil
}

// Older agents get one player per call, and no temporary bans, which they would make for good.
func TestChangeOnOlderAgents(t *testing.T) {
	agent := &olderAgent{}
	ban := func(name string, ends int64) *noryxv1.PlayerChange {
		return &noryxv1.PlayerChange{Action: noryxv1.PlayerAction_PLAYER_ACTION_BAN, Name: name, EndsUnix: ends}
	}
	got, err := changeOn(t.Context(), agent, lobby, []*noryxv1.PlayerChange{ban("Alex", 0), ban("Nobody", 0), ban("Steve", 0)})
	if err != nil || len(got) != 3 || got[0].GetOutput() != "Banned Alex" || got[1].GetError() != "The server doesn't run." || got[2].GetError() != "" {
		t.Fatalf("results = %v, %v", got, err)
	}
	if _, err := changeOn(t.Context(), agent, lobby, []*noryxv1.PlayerChange{ban("Kai", 0), ban("Bo", 1)}); err == nil || len(agent.changed) != 2 {
		t.Fatalf("temporary ban on an older agent: %v, changed %q", err, agent.changed)
	}
}
