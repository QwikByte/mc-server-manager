package pki

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// SaveKeyPair atomically writes certificate and private key together to <name>.pem in
// dir, readable by the owner only, so a renewal can never leave a mismatched pair.
func SaveKeyPair(dir, name string, certDER []byte, key crypto.Signer) error {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	data = append(data, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})...)
	if err := writeFile(dir, name+".pem", data); err != nil {
		return err
	}
	// The former layout kept the pair in two files; its key must not outlive the renewal.
	return errors.Join(removeIfExists(filepath.Join(dir, name+".crt")), removeIfExists(filepath.Join(dir, name+".key")))
}

// LoadKeyPair reads <name>.pem from dir, or <name>.crt and <name>.key written by earlier versions.
func LoadKeyPair(dir, name string) (tls.Certificate, error) {
	path := filepath.Join(dir, name+".pem")
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return tls.LoadX509KeyPair(filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key"))
	}
	return tls.LoadX509KeyPair(path, path)
}

// hasKeyPair reports whether dir contains a key pair called name in any layout.
func hasKeyPair(dir, name string) bool {
	for _, file := range []string{name + ".pem", name + ".crt"} {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}
	return false
}

// SaveCert writes <name>.crt to dir.
func SaveCert(dir, name string, der []byte) error {
	return writeFile(dir, name+".crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
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

// writeFile replaces dir/file atomically with an owner-only file.
func writeFile(dir, file string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, file+".tmp*") // created with mode 0600
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // nothing left to remove after the rename
	_, err = tmp.Write(data)
	if err = errors.Join(err, tmp.Sync(), tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, file))
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
