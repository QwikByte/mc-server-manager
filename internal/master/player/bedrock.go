package player

import (
	"context"
	"strings"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

// BedrockPlayers finds the IDs Floodgate gives Bedrock players, e.g. GeyserMC's global API.
type BedrockPlayers interface {
	PlayerID(ctx context.Context, gamertag string) (string, error)
}

// identify gives a change that whitelists a Bedrock player their ID, as the servers behind a
// proxy can't look them up. Floodgate names them by their gamertag, with a dot in front and
// underscores for spaces.
func (s *Service) identify(ctx context.Context, c *noryxv1.PlayerChange) (err error) {
	if noryxv1.BedrockPlayer(c.GetName()) && c.GetAction() == noryxv1.PlayerAction_PLAYER_ACTION_WHITELIST_ADD {
		c.Uuid, err = s.bedrock.PlayerID(ctx, strings.ReplaceAll(c.GetName()[1:], "_", " "))
	}
	return err
}
