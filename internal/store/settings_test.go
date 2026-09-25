package store

import (
	"context"
	"testing"
)

// A fresh install has no folder yet; the UI asks for one instead of guessing.
func TestEnvFolderEmptyUntilSet(tester *testing.T) {
	folder, err := openTestStore(tester).EnvFolder(context.Background())
	if err != nil || folder != "" {
		tester.Fatalf("got %q, %v", folder, err)
	}
}

// Changing the folder on the web must replace the old one, not fail on the existing row.
func TestSetEnvFolderReplacesPreviousValue(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()
	for _, folder := range []string{"/host/env", "/host/other-env"} {
		if err := testStore.SetEnvFolder(ctx, folder); err != nil {
			tester.Fatal(err)
		}
		if saved, err := testStore.EnvFolder(ctx); err != nil || saved != folder {
			tester.Fatalf("got %q, %v, want %q", saved, err, folder)
		}
	}
}

func TestSettingsFailAfterClose(tester *testing.T) {
	testStore := openTestStore(tester)
	testStore.Close()
	ctx := context.Background()
	if _, err := testStore.EnvFolder(ctx); err == nil {
		tester.Error("EnvFolder should fail")
	}
	if err := testStore.SetEnvFolder(ctx, "/env"); err == nil {
		tester.Error("SetEnvFolder should fail")
	}
}
