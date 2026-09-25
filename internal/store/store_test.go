package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
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
}
