package https

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"golang.org/x/crypto/acme"
)

// The self-signed certificate covers the machine and the given names, and stays until it lacks one.
func TestSelfSigned(t *testing.T) {
	dir := t.TempDir()
	s, err := New(SelfSigned, "panel.example", []string{"203.0.113.7"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	c := s.Certificate()
	for _, name := range []string{"panel.example", "203.0.113.7", "localhost", "127.0.0.1"} {
		if !slices.Contains(c.Names, name) {
			t.Errorf("certificate names %v lack %s", c.Names, name)
		}
	}
	if !c.SelfSigned || c.Fingerprint == "" || s.Trusted() {
		t.Errorf("certificate = %+v, trusted %v", c, s.Trusted())
	}
	if cert, _ := s.TLSConfig().GetCertificate(&tls.ClientHelloInfo{}); cert != s.fallback {
		t.Error("a client without server name doesn't get the self-signed certificate")
	}

	same, err := New(SelfSigned, "", []string{"203.0.113.7"}, dir)
	if err != nil || same.Certificate().Fingerprint != c.Fingerprint {
		t.Errorf("a certificate covering all names was replaced: %v", err)
	}
	other, err := New(SelfSigned, "other.example", nil, dir)
	if err != nil || other.Certificate().Fingerprint == c.Fingerprint || !slices.Contains(other.Certificate().Names, "other.example") {
		t.Errorf("a certificate without the new domain stayed: %v", err)
	}
}

// With Let's Encrypt, the panel serves its certificate for the domain only, the self-signed one
// for other names and after a failure, without asking Let's Encrypt again at once.
func TestLetsEncrypt(t *testing.T) {
	s, err := New(LetsEncrypt, "panel.example", nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !s.Trusted() || !slices.Contains(s.TLSConfig().NextProtos, acme.ALPNProto) {
		t.Error("Let's Encrypt can't check the domain at port 443")
	}
	calls, fail := 0, errors.New("no DNS record")
	s.issue = func(*tls.ClientHelloInfo) (*tls.Certificate, error) { calls++; return nil, fail }
	get := s.TLSConfig().GetCertificate

	if cert, _ := get(&tls.ClientHelloInfo{ServerName: "203.0.113.7"}); cert != s.fallback || calls != 0 {
		t.Errorf("another name: Let's Encrypt asked %d times", calls)
	}
	for range 2 {
		if cert, err := get(&tls.ClientHelloInfo{ServerName: "Panel.Example."}); cert != s.fallback || err != nil {
			t.Errorf("after a failure: %v", err)
		}
	}
	if c := s.Certificate(); calls != 1 || !c.SelfSigned || c.Error != fail.Error() {
		t.Errorf("Let's Encrypt asked %d times, certificate = %+v", calls, c)
	}

	issued := s.fallback // stands in for one of Let's Encrypt
	s.issue = func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return issued, nil }
	if _, err := s.obtain(&tls.ClientHelloInfo{ServerName: "panel.example"}); err != nil {
		t.Fatal(err)
	}
	if c := s.Certificate(); c.SelfSigned || c.Error != "" {
		t.Errorf("certificate = %+v", c)
	}
}

// Port 80 sends browsers to HTTPS at the domain, whichever host they asked for.
func TestRedirect(t *testing.T) {
	for _, host := range []string{"panel.example", "203.0.113.7:80", "evil.example"} {
		r := httptest.NewRequest(http.MethodGet, "/servers?q=1", nil)
		r.Host = host
		rec := httptest.NewRecorder()
		redirect("https://panel.example:8443").ServeHTTP(rec, r)
		if got, want := rec.Header().Get("Location"), "https://panel.example:8443/servers?q=1"; got != want || rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s: %d to %q, want %q", host, rec.Code, got, want)
		}
	}
}
