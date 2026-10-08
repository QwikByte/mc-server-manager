package notify

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/logs"
)

const (
	maxRules      = 200
	maxCategories = 32
)

var (
	errRuleNotFound = httpapi.Errorf(http.StatusNotFound, "Notification rule not found.")
	categoryPattern = regexp.MustCompile(`^[a-z][a-z-]{0,31}$`)
	idPattern       = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
)

// Rule sends the new entries of the log of at least a level to a channel: those of its
// categories, or all if it has none, and of its node and server if it names them.
type Rule struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelId"`
	Enabled   bool   `json:"enabled"`
	// Level is info, warn or error.
	Level      string    `json:"level"`
	Categories []string  `json:"categories"`
	NodeID     string    `json:"nodeId"`
	ServerID   string    `json:"serverId"`
	CreatedAt  time.Time `json:"createdAt"`

	level slog.Level
}

// RuleInput is a new or changed rule.
type RuleInput struct {
	ChannelID  string   `json:"channelId"`
	Enabled    bool     `json:"enabled"`
	Level      string   `json:"level"`
	Categories []string `json:"categories"`
	NodeID     string   `json:"nodeId"`
	ServerID   string   `json:"serverId"`
}

func (in RuleInput) build() (Rule, error) {
	r := Rule{ChannelID: in.ChannelID, Enabled: in.Enabled, Level: in.Level, NodeID: in.NodeID, ServerID: in.ServerID, Categories: []string{}}
	var err error
	if r.level, err = logging.ParseLevel(r.Level); err != nil || r.level < slog.LevelInfo {
		return r, httpapi.Errorf(http.StatusBadRequest, "Choose the level info, warn or error.")
	}
	for _, c := range in.Categories {
		if !categoryPattern.MatchString(c) {
			return r, httpapi.Errorf(http.StatusBadRequest, "%q isn't a category of the log.", c)
		}
		if !slices.Contains(r.Categories, c) {
			r.Categories = append(r.Categories, c)
		}
	}
	switch {
	case len(r.Categories) > maxCategories:
		return r, httpapi.Errorf(http.StatusBadRequest, "Choose at most %d categories.", maxCategories)
	case r.NodeID != "" && !idPattern.MatchString(r.NodeID), r.ServerID != "" && !idPattern.MatchString(r.ServerID):
		return r, httpapi.Errorf(http.StatusBadRequest, "The node or server is invalid.")
	case r.ServerID != "" && r.NodeID == "":
		return r, httpapi.Errorf(http.StatusBadRequest, "Choose the node of the server.")
	}
	return r, nil
}

// matches reports whether the rule sends an entry.
func (r Rule) matches(e logs.Entry) bool {
	return r.Enabled && e.Level >= r.level && (len(r.Categories) == 0 || slices.Contains(r.Categories, e.Category)) &&
		(r.NodeID == "" || r.NodeID == e.NodeID) && (r.ServerID == "" || r.ServerID == e.ServerID)
}

const ruleColumns = `id, channel_id, enabled, level, categories, node_id, server_id, created_at`

func scanRule(row interface{ Scan(...any) error }) (Rule, error) {
	var (
		r          Rule
		categories string
		at         int64
	)
	if err := row.Scan(&r.ID, &r.ChannelID, &r.Enabled, &r.Level, &categories, &r.NodeID, &r.ServerID, &at); err != nil {
		return r, err
	}
	r.CreatedAt = time.Unix(at, 0)
	r.level, _ = logging.ParseLevel(r.Level)
	return r, json.Unmarshal([]byte(categories), &r.Categories)
}

// Rules returns the rules, ordered by channel.
func (s *Service) Rules(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+ruleColumns+` FROM notification_rules ORDER BY channel_id, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := []Rule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Service) rule(ctx context.Context, id string) (Rule, error) {
	r, err := scanRule(s.db.QueryRowContext(ctx, `SELECT `+ruleColumns+` FROM notification_rules WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		err = errRuleNotFound
	}
	return r, err
}

// CreateRule stores a new rule.
func (s *Service) CreateRule(ctx context.Context, in RuleInput) (Rule, error) {
	r, err := in.build()
	if err != nil {
		return r, err
	}
	s.changes.Lock()
	defer s.changes.Unlock()
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_rules`).Scan(&n); err != nil {
		return r, err
	}
	if n >= maxRules {
		return r, httpapi.Errorf(http.StatusConflict, "The master keeps at most %d notification rules.", maxRules)
	}
	if _, err := s.channel(ctx, r.ChannelID); err != nil {
		return r, err
	}
	r.ID, r.CreatedAt = strings.ToLower(rand.Text()), time.Now()
	categories, _ := json.Marshal(r.Categories) // can't fail for strings
	if _, err := s.db.ExecContext(ctx, `INSERT INTO notification_rules (`+ruleColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ChannelID, r.Enabled, r.Level, string(categories), r.NodeID, r.ServerID, r.CreatedAt.Unix()); err != nil {
		return r, err
	}
	if err := s.reload(ctx); err != nil {
		return r, err
	}
	return s.rule(ctx, r.ID)
}

// UpdateRule changes a rule.
func (s *Service) UpdateRule(ctx context.Context, id string, in RuleInput) (Rule, error) {
	r, err := in.build()
	if err != nil {
		return r, err
	}
	s.changes.Lock()
	defer s.changes.Unlock()
	if _, err := s.channel(ctx, r.ChannelID); err != nil {
		return r, err
	}
	categories, _ := json.Marshal(r.Categories) // can't fail for strings
	res, err := s.db.ExecContext(ctx, `UPDATE notification_rules SET channel_id = ?, enabled = ?, level = ?, categories = ?, node_id = ?, server_id = ?
		WHERE id = ?`, r.ChannelID, r.Enabled, r.Level, string(categories), r.NodeID, r.ServerID, id)
	if err != nil {
		return r, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return r, errRuleNotFound
	}
	if err := s.reload(ctx); err != nil {
		return r, err
	}
	return s.rule(ctx, id)
}

// DeleteRule deletes a rule.
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	s.changes.Lock()
	defer s.changes.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM notification_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errRuleNotFound
	}
	return s.reload(ctx)
}
