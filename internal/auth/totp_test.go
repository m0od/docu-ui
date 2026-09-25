package auth

import (
	"encoding/base32"
	"testing"
	"time"
)

// Secret and expected codes come from RFC 6238 Appendix B (SHA-1), cut to 6 digits.
// If these fail, codes from real authenticator apps will not match either.
var rfcSecret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))

func TestVerifyTOTPMatchesRFCVectors(tester *testing.T) {
	rfcVectors := map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"}
	for unixSeconds, expectedCode := range rfcVectors {
		if !VerifyTOTP(rfcSecret, expectedCode, time.Unix(unixSeconds, 0)) {
			tester.Errorf("t=%d: code %s rejected", unixSeconds, expectedCode)
		}
	}
}

// A phone clock 30s off must still work; a code from 2 minutes ago must not (replay window).
func TestVerifyTOTPClockSkew(tester *testing.T) {
	codeTime := time.Unix(1111111109, 0)
	if !VerifyTOTP(rfcSecret, "081804", codeTime.Add(30*time.Second)) {
		tester.Error("one step of skew should be accepted")
	}
	if VerifyTOTP(rfcSecret, "081804", codeTime.Add(2*time.Minute)) {
		tester.Error("a code four steps old must be rejected")
	}
}

func TestVerifyTOTPRejectsBadInput(tester *testing.T) {
	now := time.Unix(59, 0)
	if VerifyTOTP("not base32!", "287082", now) {
		tester.Error("invalid secret accepted")
	}
	if VerifyTOTP(rfcSecret, "28708", now) {
		tester.Error("5-digit code accepted")
	}
	if VerifyTOTP(rfcSecret, "000000", now) {
		tester.Error("wrong code accepted")
	}
}

func TestTOTPSecrets(tester *testing.T) {
	generatedSecret := NewTOTPSecret()
	if len(generatedSecret) != 32 || !ValidTOTPSecret(generatedSecret) {
		tester.Fatalf("generated secret %q is not a valid 160-bit base32 secret", generatedSecret)
	}
	if ValidTOTPSecret("ABCDEFGH") {
		tester.Error("40-bit secret is too short to accept")
	}
	if ValidTOTPSecret("not base32!") {
		tester.Error("non-base32 secret accepted")
	}
}
