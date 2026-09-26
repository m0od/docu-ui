package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionLifecycle(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	now := time.Unix(1_000_000, 0)
	if err := testStore.CreateSession(ctx, "hash-1", admin.ID, now, now.Add(time.Hour)); err != nil {
		tester.Fatal(err)
	}
	if username, err := testStore.FindSessionUsername(ctx, "hash-1", now); err != nil || username != "admin" {
		tester.Fatalf("live session: %q %v", username, err)
	}
	// Expired sessions must not authenticate, even before cleanup removes them.
	if _, err := testStore.FindSessionUsername(ctx, "hash-1", now.Add(time.Hour)); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("expired session: %v", err)
	}
	if err := testStore.DeleteSession(ctx, "hash-1"); err != nil {
		tester.Fatal(err)
	}
	if _, err := testStore.FindSessionUsername(ctx, "hash-1", now); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("signed-out session still valid: %v", err)
	}
}

// Creating a session sweeps expired ones, or every sign-in would leave a row behind forever.
func TestCreateSessionDropsExpired(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	now := time.Unix(1_000_000, 0)
	testStore.CreateSession(ctx, "old", admin.ID, now, now.Add(time.Minute))
	testStore.CreateSession(ctx, "new", admin.ID, now.Add(time.Hour), now.Add(2*time.Hour))
	var sessionCount int
	testStore.database.QueryRow(`SELECT count(*) FROM sessions`).Scan(&sessionCount)
	if sessionCount != 1 {
		tester.Fatalf("got %d sessions, want only the new one", sessionCount)
	}
}

func TestSessionQueriesFailAfterClose(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	testStore.Close()
	ctx := context.Background()
	now := time.Now()
	if err := testStore.CreateSession(ctx, "hash", admin.ID, now, now); err == nil {
		tester.Error("CreateSession should fail")
	}
	if _, err := testStore.FindSessionUsername(ctx, "hash", now); err == nil || errors.Is(err, ErrSessionNotFound) {
		tester.Errorf("FindSessionUsername: %v", err)
	}
	if err := testStore.DeleteSession(ctx, "hash"); err == nil {
		tester.Error("DeleteSession should fail")
	}
	if err := testStore.DeleteOtherSessions(ctx, admin.ID, "hash"); err == nil {
		tester.Error("DeleteOtherSessions should fail")
	}
}

// After a password change, a stolen session elsewhere stops working; the one in use keeps going.
func TestDeleteOtherSessionsKeepsTheCurrentOne(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	now := time.Now()
	for _, tokenHash := range []string{"current", "stolen"} {
		if err := testStore.CreateSession(ctx, tokenHash, admin.ID, now, now.Add(time.Hour)); err != nil {
			tester.Fatal(err)
		}
	}
	if err := testStore.DeleteOtherSessions(ctx, admin.ID, "current"); err != nil {
		tester.Fatal(err)
	}
	if _, err := testStore.FindSessionUsername(ctx, "current", now); err != nil {
		tester.Fatalf("current: %v", err)
	}
	if _, err := testStore.FindSessionUsername(ctx, "stolen", now); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("stolen: %v", err)
	}
}
