package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordRoundTrip(tester *testing.T) {
	storedHash := HashPassword("correct horse battery")
	if !strings.HasPrefix(storedHash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		tester.Fatalf("unexpected hash format %q", storedHash)
	}
	if matched, err := VerifyPassword("correct horse battery", storedHash); err != nil || !matched {
		tester.Fatalf("right password rejected: %v %v", matched, err)
	}
	if matched, err := VerifyPassword("wrong password", storedHash); err != nil || matched {
		tester.Fatalf("wrong password accepted: %v %v", matched, err)
	}
}

// Same password twice must not give the same hash, or equal passwords would be visible in the database.
func TestHashIsSalted(tester *testing.T) {
	if HashPassword("same password") == HashPassword("same password") {
		tester.Fatal("two hashes of one password are identical: salt is missing")
	}
}

// A corrupted row must be reported, never treated as "wrong password" or, worse, a match.
func TestVerifyRejectsMalformedHash(tester *testing.T) {
	malformedHashes := map[string]string{
		"not a hash":    "plain text",
		"other algo":    "$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$a2V5",
		"bad version":   "$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$a2V5",
		"no version":    "$argon2id$x$m=19456,t=2,p=1$c2FsdA$a2V5",
		"bad params":    "$argon2id$v=19$m=x,t=2,p=1$c2FsdA$a2V5",
		"bad salt":      "$argon2id$v=19$m=19456,t=2,p=1$!!$a2V5",
		"bad key bytes": "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$!!",
	}
	for caseName, malformedHash := range malformedHashes {
		if matched, err := VerifyPassword("anything", malformedHash); matched || !errors.Is(err, ErrMalformedHash) {
			tester.Errorf("%s: got %v %v", caseName, matched, err)
		}
	}
}

func TestRandomTokenIsUnique(tester *testing.T) {
	firstToken, secondToken := RandomToken(16), RandomToken(16)
	if firstToken == secondToken || len(firstToken) != 22 {
		tester.Fatalf("got %q and %q", firstToken, secondToken)
	}
}
