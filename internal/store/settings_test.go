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
	if _, err := testStore.Webhook(ctx); err == nil {
		tester.Error("Webhook should fail")
	}
	if err := testStore.SetWebhook(ctx, Webhook{}); err == nil {
		tester.Error("SetWebhook should fail")
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

// Every field round-trips, and saving again replaces all of them (a cleared header stays cleared).
func TestWebhookSettings(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()
	if webhook, err := testStore.Webhook(ctx); err != nil || webhook != (Webhook{}) {
		tester.Fatalf("fresh install: %+v, %v", webhook, err)
	}
	for _, webhook := range []Webhook{
		{URL: "https://ci/hook", Secret: "s1", HeaderName: "Authorization", HeaderValue: "Bearer t"},
		{URL: "https://other/hook", Secret: "s2"},
	} {
		if err := testStore.SetWebhook(ctx, webhook); err != nil {
			tester.Fatal(err)
		}
		if saved, err := testStore.Webhook(ctx); err != nil || saved != webhook {
			tester.Fatalf("got %+v, %v, want %+v", saved, err, webhook)
		}
	}
}
