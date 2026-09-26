package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(tester *testing.T) *Store {
	tester.Helper()
	testStore, err := Open(filepath.Join(tester.TempDir(), "docu-ui.db"))
	if err != nil {
		tester.Fatal(err)
	}
	tester.Cleanup(func() { testStore.Close() })
	return testStore
}

// Setup must be possible exactly once: a second call would let anyone who reaches the page add an admin.
func TestFirstAccountOnlyOnce(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()

	if needsSetup, err := testStore.NeedsSetup(ctx); err != nil || !needsSetup {
		tester.Fatalf("fresh database should need setup: %v %v", needsSetup, err)
	}
	if err := testStore.CreateFirstAccount(ctx, "admin", "hash", ""); err != nil {
		tester.Fatal(err)
	}
	if needsSetup, err := testStore.NeedsSetup(ctx); err != nil || needsSetup {
		tester.Fatalf("setup should be closed after the first account: %v %v", needsSetup, err)
	}
	if err := testStore.CreateFirstAccount(ctx, "intruder", "hash", ""); !errors.Is(err, ErrSetupDone) {
		tester.Fatalf("second account: got %v, want ErrSetupDone", err)
	}
}

// The account must survive a restart, or setup would reopen every time the container restarts.
func TestAccountPersistsAcrossReopen(tester *testing.T) {
	databasePath := filepath.Join(tester.TempDir(), "docu-ui.db")
	firstStore, err := Open(databasePath)
	if err != nil {
		tester.Fatal(err)
	}
	if err := firstStore.CreateFirstAccount(context.Background(), "admin", "hash", "SECRET"); err != nil {
		tester.Fatal(err)
	}
	firstStore.Close()

	reopenedStore, err := Open(databasePath)
	if err != nil {
		tester.Fatal(err)
	}
	defer reopenedStore.Close()
	if needsSetup, _ := reopenedStore.NeedsSetup(context.Background()); needsSetup {
		tester.Fatal("setup reopened after restart")
	}
}

func TestOpenFailsOnUnwritablePath(tester *testing.T) {
	if _, err := Open(filepath.Join(tester.TempDir(), "missing-dir", "docu-ui.db")); err == nil {
		tester.Fatal("expected error for a directory that does not exist")
	}
}

func TestQueriesFailAfterClose(tester *testing.T) {
	closedStore := openTestStore(tester)
	closedStore.Close()
	if _, err := closedStore.NeedsSetup(context.Background()); err == nil {
		tester.Error("NeedsSetup on closed store should fail")
	}
	if err := closedStore.CreateFirstAccount(context.Background(), "admin", "hash", ""); err == nil {
		tester.Error("CreateFirstAccount on closed store should fail")
	}
	if _, err := closedStore.SignInOff(context.Background()); err == nil {
		tester.Error("SignInOff on closed store should fail")
	}
	if err := closedStore.SkipSignIn(context.Background()); err == nil {
		tester.Error("SkipSignIn on closed store should fail")
	}
	if err := closedStore.TurnOffSignIn(context.Background()); err == nil {
		tester.Error("TurnOffSignIn on closed store should fail")
	}
}

func signInState(tester *testing.T, testStore *Store) (needsSetup, signInOff bool) {
	tester.Helper()
	needsSetup, err := testStore.NeedsSetup(context.Background())
	if err != nil {
		tester.Fatal(err)
	}
	signInOff, err = testStore.SignInOff(context.Background())
	if err != nil {
		tester.Fatal(err)
	}
	return needsSetup, signInOff
}

// Skipping sign-in closes setup (no token-less account creation later) and opens the UI.
func TestSkipSignInClosesSetup(tester *testing.T) {
	testStore := openTestStore(tester)
	if needsSetup, signInOff := signInState(tester, testStore); !needsSetup || signInOff {
		tester.Fatalf("fresh database: needsSetup=%v signInOff=%v", needsSetup, signInOff)
	}
	if err := testStore.SkipSignIn(context.Background()); err != nil {
		tester.Fatal(err)
	}
	if needsSetup, signInOff := signInState(tester, testStore); needsSetup || !signInOff {
		tester.Fatalf("after skip: needsSetup=%v signInOff=%v", needsSetup, signInOff)
	}
}

// Skipping must not open a Docu-UI that already has an account.
func TestSkipSignInRefusedOnceAnAccountExists(tester *testing.T) {
	testStore := openTestStore(tester)
	if err := testStore.CreateFirstAccount(context.Background(), "admin", "hash", ""); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.SkipSignIn(context.Background()); !errors.Is(err, ErrSetupDone) {
		tester.Fatalf("got %v, want ErrSetupDone", err)
	}
	if _, signInOff := signInState(tester, testStore); signInOff {
		tester.Fatal("an account exists, sign-in must stay on")
	}
}

// Turning sign-in on again is creating an account; the old "off" must not count any more.
func TestAnAccountTurnsSignInBackOn(tester *testing.T) {
	testStore := openTestStore(tester)
	if err := testStore.SkipSignIn(context.Background()); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.CreateFirstAccount(context.Background(), "admin", "hash", ""); err != nil {
		tester.Fatal(err)
	}
	if needsSetup, signInOff := signInState(tester, testStore); needsSetup || signInOff {
		tester.Fatalf("needsSetup=%v signInOff=%v", needsSetup, signInOff)
	}
}

// Turning sign-in off removes the account and every session, so no old cookie still works.
func TestTurnOffSignInRemovesAccountsAndSessions(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()
	if err := testStore.CreateFirstAccount(ctx, "admin", "hash", ""); err != nil {
		tester.Fatal(err)
	}
	admin, _ := testStore.FindAccount(ctx, "admin")
	now := time.Now()
	if err := testStore.CreateSession(ctx, "hash-1", admin.ID, now, now.Add(time.Hour)); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.TurnOffSignIn(ctx); err != nil {
		tester.Fatal(err)
	}
	if _, err := testStore.FindAccount(ctx, "admin"); !errors.Is(err, ErrAccountNotFound) {
		tester.Fatalf("account: %v", err)
	}
	if _, err := testStore.FindSessionUsername(ctx, "hash-1", now); !errors.Is(err, ErrSessionNotFound) {
		tester.Fatalf("session: %v", err)
	}
	if needsSetup, signInOff := signInState(tester, testStore); needsSetup || !signInOff {
		tester.Fatalf("needsSetup=%v signInOff=%v", needsSetup, signInOff)
	}
}

// If deleting the accounts fails after the setting was written, sign-in must still be required.
func TestTurnOffSignInFailingHalfwayKeepsSignIn(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()
	if err := testStore.CreateFirstAccount(ctx, "admin", "hash", ""); err != nil {
		tester.Fatal(err)
	}
	// A trigger stands in for a delete that fails (disk full, locked database).
	if _, err := testStore.database.ExecContext(ctx,
		`CREATE TRIGGER keep_accounts BEFORE DELETE ON accounts BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.TurnOffSignIn(ctx); err == nil {
		tester.Fatal("expected the delete to fail")
	}
	if needsSetup, signInOff := signInState(tester, testStore); needsSetup || signInOff {
		tester.Fatalf("needsSetup=%v signInOff=%v", needsSetup, signInOff)
	}
}
