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
	if _, err := testStore.DocoCD(ctx); err == nil {
		tester.Error("DocoCD should fail")
	}
	if err := testStore.SetDocoCD(ctx, DocoCD{}); err == nil {
		tester.Error("SetDocoCD should fail")
	}
}

// Saving new Doco-CD settings replaces both fields, including a key cleared on purpose.
func TestDocoCDSettings(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()
	if settings, err := testStore.DocoCD(ctx); err != nil || settings != (DocoCD{}) {
		tester.Fatalf("fresh install: %+v, %v", settings, err)
	}
	for _, settings := range []DocoCD{{URL: "http://doco-cd", APIKey: "key-1"}, {URL: "http://other", APIKey: ""}} {
		if err := testStore.SetDocoCD(ctx, settings); err != nil {
			tester.Fatal(err)
		}
		if saved, err := testStore.DocoCD(ctx); err != nil || saved != settings {
			tester.Fatalf("got %+v, %v, want %+v", saved, err, settings)
		}
	}
}
