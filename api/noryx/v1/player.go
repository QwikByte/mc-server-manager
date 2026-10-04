package noryxv1

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxReason is the longest reason for a kick or ban, in characters.
const MaxReason = 256

// playerName matches the names of Java players, and those of Bedrock players, which
// Floodgate starts with a dot.
var playerName = regexp.MustCompile(`^\.?[A-Za-z0-9_]{1,16}$`)

// ValidPlayerName reports whether name is the name of a player that a console command can
// take as it is: it can't name others, like @a, or add arguments.
func ValidPlayerName(name string) bool { return playerName.MatchString(name) }

// ValidReason reports whether a reason fits on the line of a console command.
func ValidReason(reason string) bool {
	return utf8.RuneCountInString(reason) <= MaxReason && !strings.ContainsFunc(reason, unicode.IsControl)
}

// Slug returns the short lower-case name, e.g. "whitelist_add" for PLAYER_ACTION_WHITELIST_ADD.
func (a PlayerAction) Slug() string { return slug(a.String(), "PLAYER_ACTION_") }

// ParsePlayerAction returns the action with the given slug, or PLAYER_ACTION_UNSPECIFIED.
func ParsePlayerAction(slug string) PlayerAction {
	return PlayerAction(PlayerAction_value["PLAYER_ACTION_"+strings.ToUpper(slug)])
}

// Global reports whether an action changes the whole server rather than a player.
func (a PlayerAction) Global() bool {
	return a == PlayerAction_PLAYER_ACTION_WHITELIST_ON || a == PlayerAction_PLAYER_ACTION_WHITELIST_OFF
}

// Problem returns a message for the operator if a change is invalid. Only kicks and bans
// take a reason, and turning the whitelist on or off no player.
func (c *PlayerChange) Problem() string {
	a := c.GetAction()
	switch {
	case a == PlayerAction_PLAYER_ACTION_UNSPECIFIED || PlayerAction_name[int32(a)] == "":
		return "Choose an action."
	case a.Global() && c.GetName() != "", !a.Global() && !ValidPlayerName(c.GetName()):
		return "Enter the name of a player: up to 16 letters, digits and underscores."
	case !ValidReason(c.GetReason()), c.GetReason() != "" && a != PlayerAction_PLAYER_ACTION_KICK && a != PlayerAction_PLAYER_ACTION_BAN:
		return "Only kicks and bans take a reason, of up to 256 characters on one line."
	}
	return ""
}
