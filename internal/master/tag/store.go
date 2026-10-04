// Package tag keeps the tags of servers, e.g. lobby or bedwars, by which the panel finds and
// groups them. Servers live on their agents, so the master keeps their tags.
package tag

import (
	"context"
	"database/sql"
	"net/http"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	MaxPerServer = 10
	maxLength    = 24
)

// Server is a server on a node.
type Server struct {
	NodeID   string `json:"nodeId"`
	ServerID string `json:"serverId"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// All returns the sorted tags of all servers that have some.
func (s *Store) All(ctx context.Context) (map[Server][]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id, server_id, tag FROM server_tags ORDER BY tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := map[Server][]string{}
	for rows.Next() {
		var srv Server
		var tag string
		if err := rows.Scan(&srv.NodeID, &srv.ServerID, &tag); err != nil {
			return nil, err
		}
		tags[srv] = append(tags[srv], tag)
	}
	return tags, rows.Err()
}

// Change adds and removes tags of servers, all or none of them.
func (s *Store) Change(ctx context.Context, servers []Server, add, remove []string) error {
	add, err := Normalize(add)
	if err == nil {
		remove, err = Normalize(remove)
	}
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	for _, srv := range servers {
		for _, tag := range remove {
			if _, err := tx.ExecContext(ctx, `DELETE FROM server_tags WHERE node_id = ? AND server_id = ? AND tag = ?`, srv.NodeID, srv.ServerID, tag); err != nil {
				return err
			}
		}
		for _, tag := range add {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO server_tags (node_id, server_id, tag) VALUES (?, ?, ?)`, srv.NodeID, srv.ServerID, tag); err != nil {
				return err
			}
		}
	}
	// Only the changed servers can have too many, as all others had few enough before.
	var tooMany bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM server_tags GROUP BY node_id, server_id HAVING COUNT(*) > ?)`, MaxPerServer).Scan(&tooMany)
	switch {
	case err != nil:
		return err
	case tooMany:
		return httpapi.Errorf(http.StatusBadRequest, "A server can have up to %d tags.", MaxPerServer)
	}
	return tx.Commit()
}

// Copy gives a copy of a server the tags of the original.
func (s *Store) Copy(ctx context.Context, from, to Server) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO server_tags (node_id, server_id, tag)
		SELECT ?, ?, tag FROM server_tags WHERE node_id = ? AND server_id = ?`, to.NodeID, to.ServerID, from.NodeID, from.ServerID)
	return err
}

// Forget deletes the tags of a deleted server.
func (s *Store) Forget(ctx context.Context, nodeID, serverID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM server_tags WHERE node_id = ? AND server_id = ?`, nodeID, serverID)
	return err
}

// Move keeps the tags of a server that moved to another node.
func (s *Store) Move(ctx context.Context, serverID, from, to string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE server_tags SET node_id = ? WHERE node_id = ? AND server_id = ?`, to, from, serverID)
	return err
}

// Normalize trims and lowercases tags, and sorts them without duplicates. Tags consist of
// letters, digits, - and _, so that they read well in the panel and in addresses.
func Normalize(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		valid := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' }
		if tag == "" || utf8.RuneCountInString(tag) > maxLength || strings.ContainsFunc(tag, func(r rune) bool { return !valid(r) }) {
			return nil, httpapi.Errorf(http.StatusBadRequest, "Tags have up to %d letters, digits, - and _.", maxLength)
		}
		out = append(out, tag)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}
