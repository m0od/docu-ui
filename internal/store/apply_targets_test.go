package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestApplyTargetMissing(tester *testing.T) {
	if _, err := openTestStore(tester).ApplyTarget(context.Background(), "a.env"); !errors.Is(err, ErrNoApplyTarget) {
		tester.Fatalf("got %v", err)
	}
}

// Changing the services or marking a new version applied replaces the whole row.
func TestSetApplyTargetReplacesPrevious(tester *testing.T) {
	testStore := openTestStore(tester)
	ctx := context.Background()
	targets := []ApplyTarget{
		{Adapter: AdapterDocoCD, Project: "textiq-dev", Services: []string{"keycloak", "iam"}, AppliedVersion: "v1"},
		// No services means the whole project; it must come back as [], not nil, for the JSON API.
		{Adapter: AdapterWebhook, Project: "textiq-dev", Services: []string{}, AppliedVersion: "v2",
			Webhook: Webhook{URL: "https://ci/hook", Secret: "s", HeaderName: "Authorization", HeaderValue: "Bearer t"}},
	}
	for _, target := range targets {
		if err := testStore.SetApplyTarget(ctx, "keycloak.env", target); err != nil {
			tester.Fatal(err)
		}
		saved, err := testStore.ApplyTarget(ctx, "keycloak.env")
		if err != nil || !reflect.DeepEqual(saved, target) {
			tester.Fatalf("got %#v, %v, want %#v", saved, err, target)
		}
	}
}

func TestApplyTargetsFailAfterClose(tester *testing.T) {
	testStore := openTestStore(tester)
	testStore.Close()
	ctx := context.Background()
	if _, err := testStore.ApplyTarget(ctx, "a.env"); err == nil || errors.Is(err, ErrNoApplyTarget) {
		tester.Errorf("ApplyTarget: %v", err)
	}
	if err := testStore.SetApplyTarget(ctx, "a.env", ApplyTarget{}); err == nil {
		tester.Error("SetApplyTarget should fail")
	}
}

// Targets saved before webhooks existed were all Doco-CD; upgrading must keep them working that way.
func TestUpgradeKeepsExistingTargetsOnDocoCD(tester *testing.T) {
	database := openRawDatabase(tester)
	releaseBeforeWebhooks := fstest.MapFS{}
	for _, fileName := range []string{"0001_accounts.sql", "0002_sign_in.sql", "0003_settings.sql", "0004_apply_targets.sql"} {
		content, err := migrationFiles.ReadFile("migrations/" + fileName)
		if err != nil {
			tester.Fatal(err)
		}
		releaseBeforeWebhooks["migrations/"+fileName] = &fstest.MapFile{Data: content}
	}
	if err := migrate(database, releaseBeforeWebhooks); err != nil {
		tester.Fatal(err)
	}
	if _, err := database.Exec(`INSERT INTO apply_targets VALUES ('a.env', 'textiq-dev', 'keycloak', 'v1')`); err != nil {
		tester.Fatal(err)
	}
	if err := migrate(database, migrationFiles); err != nil {
		tester.Fatal(err)
	}
	target, err := (&Store{database: database}).ApplyTarget(context.Background(), "a.env")
	if err != nil || target.Adapter != AdapterDocoCD || target.Project != "textiq-dev" || target.Webhook != (Webhook{}) {
		tester.Fatalf("got %+v, %v", target, err)
	}
}
