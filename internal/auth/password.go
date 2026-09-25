// Package auth holds the credential primitives: password hashing, TOTP and random tokens.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters from the OWASP password storage cheat sheet (m=19 MiB, t=2, p=1).
const (
	argonMemoryKiB  = 19 * 1024
	argonIterations = 2
	argonThreads    = 1
	argonKeyLength  = 32
	argonSaltLength = 16
)

// ErrMalformedHash means a stored hash is not in the format HashPassword writes.
var ErrMalformedHash = errors.New("malformed password hash")

// HashPassword returns an argon2id hash in the standard PHC string format,
// e.g. "$argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>".
func HashPassword(password string) string {
	salt := make([]byte, argonSaltLength)
	_, _ = rand.Read(salt) // crypto/rand.Read never returns an error
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonThreads, argonKeyLength)
	encoding := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonThreads,
		encoding.EncodeToString(salt), encoding.EncodeToString(key))
}

// VerifyPassword reports whether password matches a hash made by HashPassword.
// Parameters are read from the hash, so raising them later keeps old hashes valid.
func VerifyPassword(password, encodedHash string) (bool, error) {
	hashParts := strings.Split(encodedHash, "$")
	if len(hashParts) != 6 || hashParts[1] != "argon2id" {
		return false, ErrMalformedHash
	}
	var version int
	var memoryKiB, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(hashParts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrMalformedHash
	}
	if _, err := fmt.Sscanf(hashParts[3], "m=%d,t=%d,p=%d", &memoryKiB, &iterations, &threads); err != nil {
		return false, ErrMalformedHash
	}
	encoding := base64.RawStdEncoding
	salt, err := encoding.DecodeString(hashParts[4])
	if err != nil {
		return false, ErrMalformedHash
	}
	expectedKey, err := encoding.DecodeString(hashParts[5])
	if err != nil {
		return false, ErrMalformedHash
	}
	actualKey := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, threads, uint32(len(expectedKey)))
	return subtle.ConstantTimeCompare(actualKey, expectedKey) == 1, nil
}

// RandomToken returns a URL-safe random string carrying byteCount bytes of entropy.
func RandomToken(byteCount int) string {
	tokenBytes := make([]byte, byteCount)
	_, _ = rand.Read(tokenBytes)
	return base64.RawURLEncoding.EncodeToString(tokenBytes)
}
