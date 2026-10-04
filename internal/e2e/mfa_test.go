package e2e

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	masterapp "github.com/QwikByte/noryx/internal/master/app"
	"github.com/QwikByte/noryx/internal/master/auth"
)

func TestTwoFactorAuthentication(t *testing.T) {
	m := startMaster(t)
	svc := m.services(t)
	srv := httptest.NewTLSServer(masterapp.Handler(svc))
	t.Cleanup(srv.Close)
	const password = "a-long-enough-password"
	for _, name := range []string{"admin", "alice"} {
		user, err := svc.Users.CreateUser(t.Context(), name, password)
		check(t, err)
		check(t, svc.Access.MakeAdmin(t.Context(), user.ID))
	}
	login := func(c apiClient, username, code string, out any) {
		c.do("POST", "/api/auth/login", map[string]string{"username": username, "password": password, "code": code}, http.StatusOK, out)
	}

	// Alice turns on two-factor authentication with a code of her app.
	alice := browser(t, srv)
	login(alice, "alice", "", nil)
	var setup auth.MFASetup
	alice.do("POST", "/api/auth/mfa/setup", nil, http.StatusOK, &setup)
	var enabled struct {
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	alice.do("POST", "/api/auth/mfa", map[string]string{"password": password, "code": totpCode(t, setup.Secret, time.Now())}, http.StatusOK, &enabled)
	var status auth.MFA
	alice.do("GET", "/api/auth/mfa", nil, http.StatusOK, &status)
	if !status.Enabled || status.RecoveryCodes != len(enabled.RecoveryCodes) || status.RecoveryCodes == 0 {
		t.Fatalf("status = %+v, recovery codes %v", status, enabled.RecoveryCodes)
	}

	// Then the password alone doesn't start a session.
	phone := browser(t, srv)
	var answer struct {
		MFARequired bool `json:"mfaRequired"`
	}
	login(phone, "alice", "", &answer)
	phone.do("GET", "/api/auth/me", nil, http.StatusUnauthorized, nil)
	if !answer.MFARequired {
		t.Fatal("no code asked for")
	}
	login(phone, "alice", totpCode(t, setup.Secret, time.Now().Add(30*time.Second)), nil)
	phone.do("GET", "/api/auth/me", nil, http.StatusOK, nil)

	// Administrators turn it off for users who lost their app, but not for themselves.
	admin := browser(t, srv)
	login(admin, "admin", "", nil)
	var users []auth.Account
	admin.do("GET", "/api/users", nil, http.StatusOK, &users)
	i := slices.IndexFunc(users, func(u auth.Account) bool { return u.Username == "alice" })
	if i < 0 || !users[i].MFA {
		t.Fatalf("users = %+v", users)
	}
	path := "/api/users/" + strconv.FormatInt(users[i].ID, 10) + "/mfa"
	alice.do("DELETE", path, nil, http.StatusBadRequest, nil)
	admin.do("DELETE", path, nil, http.StatusNoContent, nil)
	alice.do("GET", "/api/auth/mfa", nil, http.StatusOK, &status)
	if status.Enabled {
		t.Fatal("still on after the reset")
	}
}

// totpCode is the code of an authenticator app at a time (RFC 6238), computed apart from the master.
func totpCode(t *testing.T, secret string, at time.Time) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	check(t, err)
	mac := hmac.New(sha1.New, key)
	mac.Write(binary.BigEndian.AppendUint64(nil, uint64(at.Unix()/30)))
	sum := mac.Sum(nil)
	n := binary.BigEndian.Uint32(sum[sum[19]&0xf:]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000)
}
