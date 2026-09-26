package store

import (
	"context"
	"testing"
)

func openStoreWithAdmin(tester *testing.T) (*Store, int64) {
	tester.Helper()
	testStore := openTestStore(tester)
	if err := testStore.CreateFirstAccount(context.Background(), "admin", "hash", "SECRET"); err != nil {
		tester.Fatal(err)
	}
	admin, err := testStore.FindAccount(context.Background(), "admin")
	if err != nil {
		tester.Fatal(err)
	}
	return testStore, admin.ID
}

func recoveryCodesLeft(tester *testing.T, testStore *Store, accountID int64) int {
	tester.Helper()
	codeCount, err := testStore.CountRecoveryCodes(context.Background(), accountID)
	if err != nil {
		tester.Fatal(err)
	}
	return codeCount
}

// Each code works exactly once: a code seen once (screen share, printout) must not open the account again.
func TestRecoveryCodeWorksOnce(tester *testing.T) {
	testStore, adminID := openStoreWithAdmin(tester)
	ctx := context.Background()
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"hash-a", "hash-b"}); err != nil {
		tester.Fatal(err)
	}
	if used, err := testStore.UseRecoveryCode(ctx, adminID, "hash-a"); err != nil || !used {
		tester.Fatalf("first use: %v %v", used, err)
	}
	if used, _ := testStore.UseRecoveryCode(ctx, adminID, "hash-a"); used {
		tester.Fatal("a used code worked again")
	}
	if used, _ := testStore.UseRecoveryCode(ctx, adminID, "hash-unknown"); used {
		tester.Fatal("an unknown code worked")
	}
	if left := recoveryCodesLeft(tester, testStore, adminID); left != 1 {
		tester.Fatalf("left: %d", left)
	}
}

// A new set makes the old one useless, e.g. after the old printout was lost.
func TestReplaceRecoveryCodesDropsTheOldSet(tester *testing.T) {
	testStore, adminID := openStoreWithAdmin(tester)
	ctx := context.Background()
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"old-a", "old-b"}); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"new-a"}); err != nil {
		tester.Fatal(err)
	}
	if used, _ := testStore.UseRecoveryCode(ctx, adminID, "old-a"); used {
		tester.Fatal("an old code still worked")
	}
	if used, _ := testStore.UseRecoveryCode(ctx, adminID, "new-a"); !used {
		tester.Fatal("the new code did not work")
	}
}

// If saving the new set fails, the old codes must already be gone, not left working.
func TestReplaceRecoveryCodesFailingInsertLeavesNoCodes(tester *testing.T) {
	testStore, adminID := openStoreWithAdmin(tester)
	ctx := context.Background()
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"old-a"}); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"dup", "dup"}); err == nil {
		tester.Fatal("a duplicate hash must fail the insert")
	}
	if left := recoveryCodesLeft(tester, testStore, adminID); left != 0 {
		tester.Fatalf("left: %d", left)
	}
}

func TestRecoveryCodeQueriesFailAfterClose(tester *testing.T) {
	testStore, adminID := openStoreWithAdmin(tester)
	testStore.Close()
	ctx := context.Background()
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"a"}); err == nil {
		tester.Error("ReplaceRecoveryCodes on closed store should fail")
	}
	if _, err := testStore.UseRecoveryCode(ctx, adminID, "a"); err == nil {
		tester.Error("UseRecoveryCode on closed store should fail")
	}
	if _, err := testStore.CountRecoveryCodes(ctx, adminID); err == nil {
		tester.Error("CountRecoveryCodes on closed store should fail")
	}
}

// Deleting the account (sign-in turned off) must take its codes along.
func TestRecoveryCodesGoWithTheAccount(tester *testing.T) {
	testStore, adminID := openStoreWithAdmin(tester)
	ctx := context.Background()
	if err := testStore.ReplaceRecoveryCodes(ctx, adminID, []string{"hash-a"}); err != nil {
		tester.Fatal(err)
	}
	if err := testStore.TurnOffSignIn(ctx); err != nil {
		tester.Fatal(err)
	}
	if left := recoveryCodesLeft(tester, testStore, adminID); left != 0 {
		tester.Fatalf("left: %d", left)
	}
}
