package auth

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/QwikByte/mc-server-manager/internal/master/database"
)

func TestPasswordHash(t *testing.T) {
	hash := hashPassword("correct horse battery staple")
	if !verifyPassword(hash, "correct horse battery staple") {
		t.Fatal("correct password rejected")
	}
	if verifyPassword(hash, "wrong password") || verifyPassword("garbage", "x") {
		t.Fatal("wrong password accepted")
	}
}

func TestLoginSessionLifecycle(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc, ctx := NewService(db), t.Context()

	if err := svc.CreateUser(ctx, "admin", "short"); err == nil {
		t.Fatal("short password accepted")
	}
	if err := svc.CreateUser(ctx, "admin", "a-long-enough-password"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, "admin", "wrong-password!"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: got %v", err)
	}
	if _, _, err := svc.Login(ctx, "nobody", "a-long-enough-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user: got %v", err)
	}

	_, token, err := svc.Login(ctx, "ADMIN", "a-long-enough-password")
	if err != nil {
		t.Fatal(err)
	}
	if user, err := svc.Authenticate(ctx, token); err != nil || user.Username != "admin" {
		t.Fatalf("Authenticate = %+v, %v", user, err)
	}
	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("session still valid after logout: %v", err)
	}
}
