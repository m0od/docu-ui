package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// RecoveryCodeCount is how many one-time codes an account gets when TOTP is turned on.
const RecoveryCodeCount = 10

// recoveryCodeLength is 10 base32 characters: 50 random bits, far beyond guessing within the lockout.
const recoveryCodeLength = 10

// NewRecoveryCodes returns RecoveryCodeCount random codes like "k7m2q-x3p4t", for when the phone is lost.
func NewRecoveryCodes() []string {
	recoveryCodes := make([]string, RecoveryCodeCount)
	for index := range recoveryCodes {
		// rand.Text is base32 (A-Z, 2-7), so the code has no easily confused 0/1/8/9.
		randomText := strings.ToLower(rand.Text()[:recoveryCodeLength])
		recoveryCodes[index] = randomText[:5] + "-" + randomText[5:]
	}
	return recoveryCodes
}

// IsRecoveryCode reports whether code looks like a recovery code rather than a 6-digit TOTP code.
// Case, spaces and dashes do not matter, so a code copied from a file or typed by hand both work.
func IsRecoveryCode(code string) bool {
	normalizedCode := normalizeRecoveryCode(code)
	if len(normalizedCode) != recoveryCodeLength {
		return false
	}
	for _, character := range normalizedCode {
		if !(character >= 'a' && character <= 'z' || character >= '2' && character <= '7') {
			return false
		}
	}
	return true
}

// HashRecoveryCode is what the database keeps. SHA-256 is enough here: unlike a password, the code
// is random with 50 bits, so a leaked hash cannot be brute-forced back.
func HashRecoveryCode(code string) string {
	codeHash := sha256.Sum256([]byte(normalizeRecoveryCode(code)))
	return hex.EncodeToString(codeHash[:])
}

func normalizeRecoveryCode(code string) string {
	return strings.NewReplacer("-", "", " ", "").Replace(strings.ToLower(code))
}
