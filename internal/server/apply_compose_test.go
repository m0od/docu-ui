package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker puts a "docker" on PATH that lists one Compose container and records its calls.
// A non-empty composeMessage makes "docker compose" fail with it.
func fakeDocker(tester *testing.T, composeMessage string) string {
	tester.Helper()
	binDirectory := tester.TempDir()
	callLog := filepath.Join(binDirectory, "calls")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + callLog + `"
case "$1" in
ps) printf '/srv/shop|/srv/shop/compose.yml|' ;;
compose) if [ -n "$FAKE_COMPOSE_MESSAGE" ]; then printf '%s' "$FAKE_COMPOSE_MESSAGE" >&2; exit 1; fi ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDirectory, "docker"), []byte(script), 0o755); err != nil {
		tester.Fatal(err)
	}
	tester.Setenv("PATH", binDirectory)
	tester.Setenv("FAKE_COMPOSE_MESSAGE", composeMessage)
	return callLog
}

func composeTargetBody(project string) string {
	return editorBody(map[string]any{"adapter": "compose", "project": project, "services": []string{"api"}})
}

// Compose finds the containers by project name, so it cannot work without one.
func TestComposeTargetNeedsAProject(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	if response := sendJSON(handler, http.MethodPut, "/api/env-files/api.env/apply-target", composeTargetBody(""), sessionCookie); response.Code != http.StatusBadRequest {
		tester.Fatalf("got %d", response.Code)
	}
}

// Apply recreates exactly the file's services, so they start with the saved values; no settings are needed.
func TestApplyThroughCompose(tester *testing.T) {
	callLog := fakeDocker(tester, "")
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	if response := sendJSON(handler, http.MethodPut, "/api/env-files/api.env/apply-target", composeTargetBody("shop-dev"), sessionCookie); response.Code != http.StatusOK ||
		!strings.Contains(response.Body.String(), `"adapter":"compose"`) {
		tester.Fatalf("target: %d %s", response.Code, response.Body.String())
	}
	version := openForEdit(tester, handler, sessionCookie)

	applied := sendJSON(handler, http.MethodPost, "/api/env-files/api.env/apply", editorBody(map[string]any{"version": version}), sessionCookie)
	if applied.Code != http.StatusOK || responseField(tester, applied, "appliedVersion") != version {
		tester.Fatalf("apply: %d %s", applied.Code, applied.Body.String())
	}
	content, _ := os.ReadFile(callLog)
	if !strings.Contains(string(content), "compose --project-name shop-dev --project-directory /srv/shop --file /srv/shop/compose.yml up --detach --force-recreate --no-deps -- api\n") {
		tester.Fatalf("calls:\n%s", content)
	}
}

// Compose's reason reaches the user, and the file stays "not applied" so they can retry.
func TestApplyThroughComposeFails(tester *testing.T) {
	fakeDocker(tester, "pull access denied for api")
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	folder := envFolderWith(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPut, "/api/env-files/api.env/apply-target", composeTargetBody("shop-dev"), sessionCookie)
	// Saved after the target was set, so not applied yet.
	if err := os.WriteFile(filepath.Join(folder, "api.env"), []byte("APP_DB_PASSWORD=changed\n"), 0o600); err != nil {
		tester.Fatal(err)
	}
	version := openForEdit(tester, handler, sessionCookie)

	failed := sendJSON(handler, http.MethodPost, "/api/env-files/api.env/apply", editorBody(map[string]any{"version": version}), sessionCookie)
	if failed.Code != http.StatusBadGateway || !strings.Contains(failed.Body.String(), "pull access denied") {
		tester.Fatalf("got %d %s", failed.Code, failed.Body.String())
	}
	state := sendJSON(handler, http.MethodGet, "/api/env-files/api.env/apply-target", "", sessionCookie)
	if responseField(tester, state, "appliedVersion") == version {
		tester.Fatal("a failed apply must not count as applied")
	}
}

// Project and service names end up as docker compose arguments, and the Docker socket is root on the
// host: a name starting with "-" would be read as a flag (--privileged, --project-directory=/).
func TestApplyTargetRefusesNamesThatLookLikeFlags(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	if saved := sendJSON(handler, http.MethodPut, "/api/env-files/api.env/apply-target", composeTargetBody("shop-dev"), sessionCookie); saved.Code != http.StatusOK {
		tester.Fatalf("valid target: %d %s", saved.Code, saved.Body.String())
	}
	flagLikeTargets := []map[string]any{
		{"project": "shop-dev", "services": []string{"--privileged"}},
		{"project": "shop-dev", "services": []string{"-d"}},
		{"project": "shop-dev", "services": []string{"api", "--build"}},
		{"project": "shop-dev", "services": []string{"--project-directory=/"}},
		{"project": "-p", "services": []string{"api"}},
		{"project": "--x", "services": []string{"api"}},
	}
	for _, target := range flagLikeTargets {
		target["adapter"] = "compose"
		if response := sendJSON(handler, http.MethodPut, "/api/env-files/api.env/apply-target", editorBody(target), sessionCookie); response.Code != http.StatusBadRequest {
			tester.Errorf("%v: got %d %s", target, response.Code, response.Body.String())
		}
	}
	saved := sendJSON(handler, http.MethodGet, "/api/env-files/api.env/apply-target", "", sessionCookie)
	if !strings.Contains(saved.Body.String(), `"project":"shop-dev"`) || !strings.Contains(saved.Body.String(), `"services":["api"]`) {
		tester.Fatalf("the saved target changed: %s", saved.Body.String())
	}
}
