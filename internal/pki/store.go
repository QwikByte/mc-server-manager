package pki

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// SaveKeyPair writes <name>.crt and <name>.key to dir, readable by the owner only.
func SaveKeyPair(dir, name string, certDER []byte, key crypto.Signer) error {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := writePEM(dir, name+".key", "PRIVATE KEY", keyDER); err != nil {
		return err
	}
	return SaveCert(dir, name, certDER)
}

// SaveCert writes <name>.crt to dir.
func SaveCert(dir, name string, der []byte) error {
	return writePEM(dir, name+".crt", "CERTIFICATE", der)
}

// LoadKeyPair reads <name>.crt and <name>.key from dir.
func LoadKeyPair(dir, name string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key"))
}

// LoadCert reads <name>.crt from dir.
func LoadCert(dir, name string) (*x509.Certificate, error) {
	path := filepath.Join(dir, name+".crt")
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the operator's configuration
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("%s: no PEM certificate found", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

func writePEM(dir, file, blockType string, der []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	return os.WriteFile(filepath.Join(dir, file), data, 0o600)
}
