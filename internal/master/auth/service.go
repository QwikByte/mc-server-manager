// Package auth manages administrator accounts and their panel sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const minPasswordLen = 12

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrNoSession          = errors.New("no valid session")

	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)
)

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

// CreateUser adds an administrator account.
func (s *Service) CreateUser(ctx context.Context, username, password string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 3-32 letters, digits, '_', '.' or '-'")
	}
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters long", minPasswordLen)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, hashPassword(password), time.Now().Unix())
	return err
}

// HasUsers reports whether at least one account exists.
func (s *Service) HasUsers(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&exists)
	return exists, err
}

// Login verifies the credentials and starts a session that lasts for ttl, identified by
// the returned token.
func (s *Service) Login(ctx context.Context, username, password string, ttl time.Duration) (User, string, error) {
	user := User{Username: username}
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		verifyPassword(dummyHash(), password)
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	if !verifyPassword(hash, password) {
		return User{}, "", ErrInvalidCredentials
	}

	now := time.Now()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		return User{}, "", err
	}
	token := rand.Text()
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashToken(token), user.ID, now.Add(ttl).Unix())
	return user, token, err
}

// Authenticate returns the user owning a valid session token.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ?`, hashToken(token), time.Now().Unix()).
		Scan(&user.ID, &user.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return user, ErrNoSession
	}
	return user, err
}

// Logout ends the session identified by token.
func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
