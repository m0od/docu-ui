package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func storeWithAdmin(tester *testing.T) (*Store, Account) {
	tester.Helper()
	testStore := openTestStore(tester)
	if err := testStore.CreateFirstAccount(context.Background(), "admin", "hash", "SECRET"); err != nil {
		tester.Fatal(err)
	}
	admin, err := testStore.FindAccount(context.Background(), "admin")
	if err != nil {
		tester.Fatal(err)
	}
	return testStore, admin
}

func TestFindAccount(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	if admin.Username != "admin" || admin.PasswordHash != "hash" || admin.TOTPSecret != "SECRET" || admin.LockedUntil.Unix() != 0 {
		tester.Fatalf("got %+v", admin)
	}
	if _, err := testStore.FindAccount(context.Background(), "nobody"); !errors.Is(err, ErrAccountNotFound) {
		tester.Fatalf("unknown user: got %v", err)
	}
}

// The fifth wrong password locks the account; before that it stays open.
func TestFailedLoginsLockAccount(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	lockUntil := time.Unix(2_000_000_000, 0)
	for failure := 1; failure <= 4; failure++ {
		testStore.RecordFailedLogin(ctx, admin.ID, 5, lockUntil)
	}
	if account, _ := testStore.FindAccount(ctx, "admin"); account.LockedUntil.Unix() != 0 {
		tester.Fatalf("locked after 4 failures: %v", account.LockedUntil)
	}
	if err := testStore.RecordFailedLogin(ctx, admin.ID, 5, lockUntil); err != nil {
		tester.Fatal(err)
	}
	if account, _ := testStore.FindAccount(ctx, "admin"); !account.LockedUntil.Equal(lockUntil) {
		tester.Fatalf("5th failure should lock until %v, got %v", lockUntil, account.LockedUntil)
	}
}

// A successful sign-in forgives earlier typos, so they do not add up across days.
func TestResetFailedLogins(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	lockUntil := time.Unix(2_000_000_000, 0)
	for failure := 1; failure <= 4; failure++ {
		testStore.RecordFailedLogin(ctx, admin.ID, 5, lockUntil)
	}
	if err := testStore.ResetFailedLogins(ctx, admin.ID); err != nil {
		tester.Fatal(err)
	}
	testStore.RecordFailedLogin(ctx, admin.ID, 5, lockUntil)
	if account, _ := testStore.FindAccount(ctx, "admin"); account.LockedUntil.Unix() != 0 {
		tester.Fatal("failures before the reset still counted")
	}
}

// A TOTP code (one time step) works once; replaying it, or an older one, must fail.
func TestUseTOTPStepRejectsReplay(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	ctx := context.Background()
	if accepted, err := testStore.UseTOTPStep(ctx, admin.ID, 100); err != nil || !accepted {
		tester.Fatalf("first use: %v %v", accepted, err)
	}
	for _, replayedStep := range []int64{100, 99} {
		if accepted, _ := testStore.UseTOTPStep(ctx, admin.ID, replayedStep); accepted {
			tester.Errorf("step %d accepted after step 100 was used", replayedStep)
		}
	}
	if accepted, _ := testStore.UseTOTPStep(ctx, admin.ID, 101); !accepted {
		tester.Error("next step rejected")
	}
}

func TestAccountQueriesFailAfterClose(tester *testing.T) {
	testStore, admin := storeWithAdmin(tester)
	testStore.Close()
	ctx := context.Background()
	if _, err := testStore.FindAccount(ctx, "admin"); err == nil || errors.Is(err, ErrAccountNotFound) {
		tester.Errorf("FindAccount: %v", err)
	}
	if err := testStore.RecordFailedLogin(ctx, admin.ID, 5, time.Now()); err == nil {
		tester.Error("RecordFailedLogin should fail")
	}
	if err := testStore.ResetFailedLogins(ctx, admin.ID); err == nil {
		tester.Error("ResetFailedLogins should fail")
	}
	if _, err := testStore.UseTOTPStep(ctx, admin.ID, 1); err == nil {
		tester.Error("UseTOTPStep should fail")
	}
}
