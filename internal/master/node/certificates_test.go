package node

import (
	"crypto/sha256"
	"crypto/x509"
	"testing"
	"time"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/pki"
)

// The master stores when the certificate of a node expires, so that it knows it after a
// restart while the node is offline, and the expiry only moves forward.
func TestCertificateExpiryIsStored(t *testing.T) {
	s := newTestService(t)
	hash := sha256.Sum256([]byte("secret"))
	if _, err := s.db.Exec(`INSERT INTO nodes (id, name, address, created_at, join_secret_hash, join_expires_at) VALUES ('n1', 'n1', '127.0.0.1:7443', 0, ?, ?)`,
		hash[:], time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	csr, err := pki.NewCSR(pki.NewKey())
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Enroll(t.Context(), &noryxv1.EnrollRequest{NodeId: "n1", Secret: "secret", CsrDer: csr})
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(res.GetCertificateDer())
	if err != nil {
		t.Fatal(err)
	}
	expiry := func(s *Service) time.Time {
		t.Helper()
		n, err := s.Get(t.Context(), "n1")
		if err != nil || n.CertificateExpiresAt == nil {
			t.Fatalf("Get = %+v, %v, want a node with the expiry of its certificate", n, err)
		}
		return *n.CertificateExpiresAt
	}
	if got := expiry(s); !got.Equal(cert.NotAfter) {
		t.Fatalf("expiry after enrolling = %v, want %v", got, cert.NotAfter)
	}

	// A restarted master doesn't take an older certificate that a node presents, but a newer one.
	restarted := NewService(s.db, s.ca, s.cert, nil)
	t.Cleanup(restarted.Close)
	restarted.seen(t.Context(), "n1", cert.NotAfter.Add(-time.Hour))
	if got := expiry(restarted); !got.Equal(cert.NotAfter) {
		t.Fatalf("expiry after an older certificate = %v, want %v", got, cert.NotAfter)
	}
	later := cert.NotAfter.Add(time.Hour)
	restarted.seen(t.Context(), "n1", later)
	if got := expiry(restarted); !got.Equal(later) {
		t.Fatalf("expiry after a newer certificate = %v, want %v", got, later)
	}
}
