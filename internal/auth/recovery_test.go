package auth

import (
	"regexp"
	"testing"
)

// Codes must be unique and random: a repeated or predictable code would be a second password.
func TestNewRecoveryCodesAreDistinctAndReadable(tester *testing.T) {
	recoveryCodes := NewRecoveryCodes()
	if len(recoveryCodes) != RecoveryCodeCount {
		tester.Fatalf("got %d codes", len(recoveryCodes))
	}
	seenCodes := map[string]bool{}
	codeFormat := regexp.MustCompile(`^[a-z2-7]{5}-[a-z2-7]{5}$`)
	for _, recoveryCode := range append(recoveryCodes, NewRecoveryCodes()...) {
		if !codeFormat.MatchString(recoveryCode) || !IsRecoveryCode(recoveryCode) {
			tester.Fatalf("bad code %q", recoveryCode)
		}
		if seenCodes[recoveryCode] {
			tester.Fatalf("code %q repeated", recoveryCode)
		}
		seenCodes[recoveryCode] = true
	}
}

// Sign-in has one code field: a 6-digit TOTP code must never be taken for a recovery code.
func TestIsRecoveryCode(tester *testing.T) {
	expectedResults := map[string]bool{
		"abcde-23456":   true,
		"ABCDE 23456":   true,
		"abcde23456":    true,
		"287082":        false,
		"abcde-2345":    false,
		"abcde-23458":   false, // 8 is not base32
		"abcde-2345!":   false,
		"abcde-234567a": false,
	}
	for code, expected := range expectedResults {
		if IsRecoveryCode(code) != expected {
			tester.Errorf("%q: want %v", code, expected)
		}
	}
}

// The hash must match however the code was typed, and differ between codes.
func TestHashRecoveryCodeIgnoresFormatting(tester *testing.T) {
	if HashRecoveryCode("abcde-23456") != HashRecoveryCode(" ABCDE 23456") {
		tester.Fatal("formatting changed the hash")
	}
	if HashRecoveryCode("abcde-23456") == HashRecoveryCode("abcde-23457") {
		tester.Fatal("two codes share a hash")
	}
}
