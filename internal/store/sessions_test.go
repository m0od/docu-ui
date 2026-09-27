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
	if username, err := testStore.UseSession(ctx, "hash-1", now, time.Hour); err != nil || username != "admin" {
		tester.Fatalf("live session: %q %v", username, err)
	}
	// Expired sessions must not authenticate, even before cleanup removes them.
	if _, err := testStore.UseSession(ctx, "hash-1", now.Add(time.Hour), time.Hour); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("expired session: %v", err)
	}
	if err := testStore.DeleteSession(ctx, "hash-1"); err != nil {
		tester.Fatal(err)
	}
	if _, err := testStore.UseSession(ctx, "hash-1", now, time.Hour); !errors.Is(err, ErrSessionNotFound) {
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
	if _, err := testStore.UseSession(ctx, "hash", now, time.Hour); err == nil || errors.Is(err, ErrSessionNotFound) {
		tester.Errorf("UseSession: %v", err)
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
	if _, err := testStore.UseSession(ctx, "current", now, time.Hour); err != nil {
		tester.Fatalf("current: %v", err)
	}
	if _, err := testStore.UseSession(ctx, "stolen", now, time.Hour); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("stolen: %v", err)
	}
}

// Idle timeout: a session left unused for idleTimeout is over, while each use pushes that limit back.
func TestSessionEndsAfterIdleTimeout(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	signedInAt := time.Unix(1_000_000, 0)
	testStore.CreateSession(ctx, "hash-1", admin.ID, signedInAt, signedInAt.Add(8*time.Hour))
	for _, usedAfter := range []time.Duration{29 * time.Minute, 58 * time.Minute} {
		if _, err := testStore.UseSession(ctx, "hash-1", signedInAt.Add(usedAfter), 30*time.Minute); err != nil {
			tester.Fatalf("used after %v: %v", usedAfter, err)
		}
	}
	// 30 minutes after the last use (at 58 min) the session is gone, and using it again does not revive it.
	for _, usedAfter := range []time.Duration{88 * time.Minute, 89 * time.Minute} {
		if _, err := testStore.UseSession(ctx, "hash-1", signedInAt.Add(usedAfter), 30*time.Minute); !errors.Is(err, ErrSessionNotFound) {
			tester.Fatalf("used after %v: %v", usedAfter, err)
		}
	}
}

// Absolute timeout: however active, a session ends at its expiry, so a stolen cookie is not good forever.
func TestActiveSessionStillExpires(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	signedInAt := time.Unix(1_000_000, 0)
	expiresAt := signedInAt.Add(8 * time.Hour)
	testStore.CreateSession(ctx, "hash-1", admin.ID, signedInAt, expiresAt)
	for usedAt := signedInAt; usedAt.Before(expiresAt); usedAt = usedAt.Add(20 * time.Minute) {
		if _, err := testStore.UseSession(ctx, "hash-1", usedAt, 30*time.Minute); err != nil {
			tester.Fatalf("at %v: %v", usedAt.Sub(signedInAt), err)
		}
	}
	if _, err := testStore.UseSession(ctx, "hash-1", expiresAt, 30*time.Minute); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("at expiry: %v", err)
	}
}
