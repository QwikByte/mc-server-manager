package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters as recommended by OWASP (19 MiB memory, 2 iterations, 1 thread).
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
)

var b64 = base64.RawStdEncoding

// hashPassword returns an encoded hash in the PHC string format.
func hashPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key))
}

// verifyPassword checks password against a hash created by hashPassword.
func verifyPassword(encoded, password string) bool {
	var version int
	var memory, iterations uint32
	var threads uint8
	var rest string
	_, err := fmt.Sscanf(encoded, "$argon2id$v=%d$m=%d,t=%d,p=%d$%s", &version, &memory, &iterations, &threads, &rest)
	saltB64, keyB64, ok := strings.Cut(rest, "$")
	if err != nil || !ok || version != argon2.Version {
		return false
	}
	salt, err1 := b64.DecodeString(saltB64)
	want, err2 := b64.DecodeString(keyB64)
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is verified for unknown usernames so that login timing does not reveal them.
var dummyHash = sync.OnceValue(func() string { return hashPassword(rand.Text()) })
