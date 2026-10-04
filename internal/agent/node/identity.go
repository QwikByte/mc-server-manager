package node

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"sync"

	"github.com/QwikByte/noryx/internal/agent/enroll"
	"github.com/QwikByte/noryx/internal/pki"
)

// Identity is the certificate the agent presents to the master. The master renews it
// before it expires: the agent creates a fresh key and signing request, the master
// signs it, and the agent installs the certificate after checking it.
type Identity struct {
	*pki.Holder
	CA  *x509.Certificate
	dir string

	mu      sync.Mutex
	pending ed25519.PrivateKey
}

// LoadIdentity reads the credentials written by enrollment.
func LoadIdentity(dir string) (*Identity, error) {
	cert, err := pki.LoadKeyPair(dir, enroll.NodeCert)
	if err != nil {
		return nil, err
	}
	ca, err := pki.LoadCert(dir, enroll.CACert)
	if err != nil {
		return nil, err
	}
	holder, err := pki.NewHolder(cert)
	return &Identity{Holder: holder, CA: ca, dir: dir}, err
}

func (id *Identity) createCSR() ([]byte, error) {
	key := pki.NewKey()
	csr, err := pki.NewCSR(key)
	if err != nil {
		return nil, err
	}
	id.mu.Lock()
	defer id.mu.Unlock()
	id.pending = key
	return csr, nil
}

// install activates a certificate after checking that it comes from the agent's CA,
// carries the agent's identity and belongs to the pending key.
func (id *Identity) install(der []byte) error {
	id.mu.Lock()
	defer id.mu.Unlock()
	if id.pending == nil {
		return errors.New("no certificate signing request is pending")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	roots.AddCert(id.CA)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: id.Get().Leaf.Subject.CommonName}); err != nil {
		return err
	}
	if !id.pending.Public().(ed25519.PublicKey).Equal(cert.PublicKey) {
		return errors.New("the certificate does not belong to the pending key")
	}
	if err := pki.SaveKeyPair(id.dir, enroll.NodeCert, der, id.pending); err != nil {
		return err
	}
	if err := id.Set(tls.Certificate{Certificate: [][]byte{der}, PrivateKey: id.pending, Leaf: cert}); err != nil {
		return err
	}
	id.pending = nil
	return nil
}
