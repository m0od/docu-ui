package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
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
		{Project: "textiq-dev", Services: []string{"keycloak", "iam"}, AppliedVersion: "v1"},
		// No services means the whole project; it must come back as [], not nil, for the JSON API.
		{Project: "textiq-dev", Services: []string{}, AppliedVersion: "v2"},
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
