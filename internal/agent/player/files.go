package player

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
)

const (
	// The lists of Minecraft, which the server writes when they change.
	bannedFile    = "banned-players.json"
	whitelistFile = "whitelist.json"
	opsFile       = "ops.json"
	// cacheFile is Minecraft's cache of the players who joined.
	cacheFile = "usercache.json"
	// pendingFile keeps the changes that wait for the server to run or for their time, in its
	// data so that they move and are backed up with it.
	pendingFile = "noryx-pending-players.json"

	createdLayout = "2006-01-02 15:04:05 -0700" // of the times in the lists
	maxFileBytes  = 8 << 20
	maxListed     = 10_000
	maxPending    = 1000
)

// entry is a player in a list of Minecraft; only bans have the times, source and reason.
// Expires is "forever", or when a temporary ban ends, which Paper and plugins write.
type entry struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Created string `json:"created"`
	Source  string `json:"source"`
	Expires string `json:"expires"`
	Reason  string `json:"reason"`
}

// readList reads a list of Minecraft; a missing file is an empty list.
func readList(dir *datadir.Dir, name string) ([]*noryxv1.ListedPlayer, error) {
	var entries []entry
	if err := readJSON(dir, name, &entries); err != nil {
		return nil, err
	}
	list := make([]*noryxv1.ListedPlayer, 0, min(len(entries), maxListed))
	for _, e := range entries[:min(len(entries), maxListed)] {
		p := &noryxv1.ListedPlayer{Name: e.Name, Uuid: e.UUID, Reason: e.Reason, Source: e.Source}
		if created, err := time.Parse(createdLayout, e.Created); err == nil {
			p.CreatedUnix = created.Unix()
		}
		// Any other value than such a time counts as forever, as it does for Minecraft.
		if expires, err := time.Parse(createdLayout, e.Expires); err == nil {
			p.ExpiresUnix = expires.Unix()
		}
		list = append(list, p)
	}
	return list, nil
}

// change is a change that waits, as the file keeps it. Due is when it is due, e.g. the pardon
// at the end of a temporary ban; 0 once the server runs.
type change struct {
	Action string `json:"action"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason,omitempty"`
	UUID   string `json:"uuid,omitempty"`
	Due    int64  `json:"due,omitempty"`
}

// readPending returns the changes that wait for a server. Those that aren't valid, e.g.
// because a plugin wrote them, are left out.
func readPending(dir *datadir.Dir) ([]*noryxv1.PlayerChange, error) {
	var changes []change
	if err := readJSON(dir, pendingFile, &changes); err != nil {
		return nil, err
	}
	var pending []*noryxv1.PlayerChange
	for _, c := range changes {
		p := &noryxv1.PlayerChange{Action: noryxv1.ParsePlayerAction(c.Action), Name: c.Name, Reason: c.Reason, Uuid: c.UUID, DueUnix: c.Due}
		if p.Problem() == "" && p.GetAction() != noryxv1.PlayerAction_PLAYER_ACTION_KICK {
			pending = append(pending, p)
		}
	}
	return pending, nil
}

// notePending notes a change in the changes that wait for a server, and returns them: the
// change itself if it waits, unless it waits already, and for a ban or pardon, the pardon at
// the end of a temporary ban instead of the one of an earlier ban.
func notePending(dir *datadir.Dir, c *noryxv1.PlayerChange, waits bool) ([]*noryxv1.PlayerChange, error) {
	pending, err := readPending(dir)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	a := c.GetAction()
	noted := pending
	if a == noryxv1.PlayerAction_PLAYER_ACTION_BAN || a == noryxv1.PlayerAction_PLAYER_ACTION_PARDON {
		noted = slices.DeleteFunc(slices.Clone(pending), func(p *noryxv1.PlayerChange) bool {
			return p.GetDueUnix() > 0 && strings.EqualFold(p.GetName(), c.GetName())
		})
	}
	if waits && !slices.ContainsFunc(noted, func(p *noryxv1.PlayerChange) bool {
		return p.GetDueUnix() == 0 && p.GetAction() == a && p.GetName() == c.GetName() && p.GetReason() == c.GetReason()
	}) {
		noted = append(noted, &noryxv1.PlayerChange{Action: a, Name: c.GetName(), Reason: c.GetReason(), Uuid: c.GetUuid()})
	}
	if c.GetEndsUnix() > 0 {
		noted = append(noted, &noryxv1.PlayerChange{Action: noryxv1.PlayerAction_PLAYER_ACTION_PARDON, Name: c.GetName(), DueUnix: c.GetEndsUnix()})
	}
	switch {
	case slices.Equal(noted, pending):
		return pending, nil
	case len(noted) > maxPending:
		return nil, status.Error(codes.ResourceExhausted, "Too many changes wait for the server.")
	}
	if err := writePending(dir, noted); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return noted, nil
}

// joined is a player in Minecraft's cache of the players who joined.
type joined struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

// readJoined returns the players who joined a server, as its cache of players tells, with
// valid names only.
func readJoined(dir *datadir.Dir) ([]*noryxv1.ListedPlayer, error) {
	var entries []joined
	if err := readJSON(dir, cacheFile, &entries); err != nil {
		return nil, err
	}
	var list []*noryxv1.ListedPlayer
	for _, e := range entries {
		if noryxv1.ValidPlayerName(e.Name) && len(list) < maxListed {
			list = append(list, &noryxv1.ListedPlayer{Name: e.Name, Uuid: e.UUID})
		}
	}
	return list, nil
}

// writePending replaces the changes that wait for a server; without any, the file goes.
func writePending(dir *datadir.Dir, pending []*noryxv1.PlayerChange) error {
	if len(pending) == 0 {
		if err := dir.Remove(pendingFile); !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	changes := make([]change, len(pending))
	for i, p := range pending {
		changes[i] = change{p.GetAction().Slug(), p.GetName(), p.GetReason(), p.GetUuid(), p.GetDueUnix()}
	}
	data, err := json.MarshalIndent(changes, "", "  ")
	if err != nil {
		return err
	}
	return dir.WriteFile(pendingFile, data)
}

// readJSON reads a JSON file of a server's data into v; a missing file leaves v alone.
func readJSON(dir *datadir.Dir, name string, v any) error {
	f, err := dir.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	switch {
	case err != nil:
		return err
	case len(data) > maxFileBytes:
		return fmt.Errorf("%s is too large", name)
	case len(data) == 0:
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
