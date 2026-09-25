package compose

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDocker puts a "docker" on PATH that records its arguments, prints listing for "ps" and
// fails "compose" with composeMessage when composeMessage is set. It returns the call log.
func fakeDocker(tester *testing.T, listing, composeMessage string) string {
	tester.Helper()
	binDirectory := tester.TempDir()
	callLog := filepath.Join(binDirectory, "calls")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + callLog + `"
case "$1" in
ps) printf '%s' "$FAKE_LISTING" ;;
compose) if [ -n "$FAKE_COMPOSE_MESSAGE" ]; then printf '%s' "$FAKE_COMPOSE_MESSAGE" >&2; exit 1; fi ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDirectory, "docker"), []byte(script), 0o755); err != nil {
		tester.Fatal(err)
	}
	tester.Setenv("PATH", binDirectory)
	tester.Setenv("FAKE_LISTING", listing)
	tester.Setenv("FAKE_COMPOSE_MESSAGE", composeMessage)
	return callLog
}

func calls(tester *testing.T, callLog string) []string {
	tester.Helper()
	content, err := os.ReadFile(callLog)
	if err != nil {
		tester.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(content)), "\n")
}

// Compose must see the same files and .env it was started with, or it would build a different project.
func TestRecreateUsesTheFilesFromTheLabels(tester *testing.T) {
	callLog := fakeDocker(tester,
		"/srv/textiq|/srv/textiq/compose.yml,/srv/textiq/compose.dev.yml|/srv/textiq/.env\n/srv/textiq|/srv/textiq/compose.yml||\n", "")

	if err := Recreate(context.Background(), "textiq-dev", []string{"keycloak", "iam"}); err != nil {
		tester.Fatal(err)
	}
	expected := []string{
		`ps --all --filter label=com.docker.compose.project=textiq-dev --format {{.Label "com.docker.compose.project.working_dir"}}|{{.Label "com.docker.compose.project.config_files"}}|{{.Label "com.docker.compose.project.environment_file"}}`,
		"compose --project-name textiq-dev --project-directory /srv/textiq --file /srv/textiq/compose.yml --file /srv/textiq/compose.dev.yml --env-file /srv/textiq/.env up --detach --force-recreate --no-deps keycloak iam",
	}
	if got := calls(tester, callLog); strings.Join(got, "\n") != strings.Join(expected, "\n") {
		tester.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

// No services means every service; without a project .env no --env-file is passed.
func TestRecreateWholeProjectWithoutEnvFile(tester *testing.T) {
	callLog := fakeDocker(tester, "/srv/app|/srv/app/compose.yml|\n", "")

	if err := Recreate(context.Background(), "app", nil); err != nil {
		tester.Fatal(err)
	}
	if got := calls(tester, callLog)[1]; got != "compose --project-name app --project-directory /srv/app --file /srv/app/compose.yml up --detach --force-recreate --no-deps" {
		tester.Fatalf("got %s", got)
	}
}

// Without a running container there are no labels to find the files from.
func TestRecreateNeedsAContainer(tester *testing.T) {
	callLog := fakeDocker(tester, "", "")

	err := Recreate(context.Background(), "app", nil)
	if err == nil || !strings.Contains(err.Error(), `no container of project "app"`) {
		tester.Fatalf("got %v", err)
	}
	if got := calls(tester, callLog); len(got) != 1 {
		tester.Fatalf("compose must not run: %v", got)
	}
}

// What Compose printed is the reason the user needs to see.
func TestRecreateReportsComposeMessage(tester *testing.T) {
	fakeDocker(tester, "/srv/app|/srv/app/compose.yml|", "  service \"web\" has no image  ")

	err := Recreate(context.Background(), "app", nil)
	if err == nil || err.Error() != `docker compose failed: service "web" has no image` {
		tester.Fatalf("got %v", err)
	}
}

// Compose prints progress first and the error last; a long message keeps its end.
func TestRecreateKeepsTheEndOfALongMessage(tester *testing.T) {
	fakeDocker(tester, "/srv/app|/srv/app/compose.yml|", strings.Repeat("progress ", 1000)+"the real error")

	err := Recreate(context.Background(), "app", nil)
	if err == nil || !strings.HasSuffix(err.Error(), "the real error") || len(err.Error()) > messageLimit+40 {
		tester.Fatalf("got %d bytes", len(err.Error()))
	}
}

// Without the docker CLI (or the socket) nothing prints; the Go error explains it.
func TestRecreateWithoutDocker(tester *testing.T) {
	tester.Setenv("PATH", tester.TempDir())

	err := Recreate(context.Background(), "app", nil)
	if err == nil || !strings.HasPrefix(err.Error(), "docker ps failed: ") {
		tester.Fatalf("got %v", err)
	}
}
