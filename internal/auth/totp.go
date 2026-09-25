package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// TOTP settings every authenticator app (Google Authenticator, Authy, 1Password) uses by default.
const (
	totpPeriodSeconds = 30
	totpDigits        = 6
	// Accept one step before and after "now" so a slightly wrong phone clock still works.
	totpAllowedSkewSteps = 1
)

var totpSecretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random 160-bit secret, base32-encoded as authenticator apps expect.
func NewTOTPSecret() string {
	secretBytes := make([]byte, 20)
	_, _ = rand.Read(secretBytes)
	return totpSecretEncoding.EncodeToString(secretBytes)
}

// ValidTOTPSecret reports whether secret is base32 text of at least 128 bits.
func ValidTOTPSecret(secret string) bool {
	secretBytes, err := totpSecretEncoding.DecodeString(strings.ToUpper(secret))
	return err == nil && len(secretBytes) >= 16
}

// VerifyTOTP reports whether code is the RFC 6238 code for secret at now (± the allowed skew).
func VerifyTOTP(secret, code string, now time.Time) bool {
	secretBytes, err := totpSecretEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil || len(code) != totpDigits {
		return false
	}
	currentStep := now.Unix() / totpPeriodSeconds
	for stepOffset := int64(-totpAllowedSkewSteps); stepOffset <= totpAllowedSkewSteps; stepOffset++ {
		expectedCode := totpCode(secretBytes, uint64(currentStep+stepOffset))
		if subtle.ConstantTimeCompare([]byte(expectedCode), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// totpCode is the HOTP value (RFC 4226) for one time step.
func totpCode(secretBytes []byte, timeStep uint64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], timeStep)
	mac := hmac.New(sha1.New, secretBytes)
	mac.Write(counter[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(digest[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, truncated%1_000_000)
}
