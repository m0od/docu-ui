package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func editorBody(fields map[string]any) string {
	encodedBody, _ := json.Marshal(fields)
	return string(encodedBody)
}

// openForEdit returns the file's version as the editor would get it.
func openForEdit(tester *testing.T, handler http.Handler, sessionCookie *http.Cookie) string {
	tester.Helper()
	response := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env", "", sessionCookie)
	version, _ := responseField(tester, response, "version").(string)
	if version == "" {
		tester.Fatalf("no version: %s", response.Body.String())
	}
	return version
}

func TestTextEditorRoundTrip(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	folder := envFolderWith(tester, handler, sessionCookie)

	opened := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/content", "", sessionCookie)
	if opened.Code != http.StatusOK || responseField(tester, opened, "content") != "KC_DB=postgres\nKC_DB_PASSWORD=s3cret\n" {
		tester.Fatalf("open: %d %s", opened.Code, opened.Body.String())
	}
	// Secrets in clear: must not stay in a browser or proxy cache.
	if opened.Header().Get("Cache-Control") != "no-store" {
		tester.Fatal("content may be cached")
	}

	saved := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content", editorBody(map[string]any{
		"baseVersion": responseField(tester, opened, "version"),
		"content":     "KC_DB=mariadb\n",
	}), sessionCookie)
	if saved.Code != http.StatusOK || responseField(tester, saved, "version") == responseField(tester, opened, "version") {
		tester.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	if content, _ := os.ReadFile(filepath.Join(folder, "keycloak.env")); string(content) != "KC_DB=mariadb\n" {
		tester.Fatalf("file %q", content)
	}
	// The saving user is recorded in the history entry name.
	entries, _ := os.ReadDir(filepath.Join(folder, ".history", "keycloak.env"))
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), "_admin.env") {
		tester.Fatalf("history %v", entries)
	}
}

func TestChangeVariables(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	folder := envFolderWith(tester, handler, sessionCookie)
	version := openForEdit(tester, handler, sessionCookie)

	response := sendJSON(handler, http.MethodPatch, "/api/env-files/keycloak.env/variables", editorBody(map[string]any{
		"baseVersion": version,
		"changes": []map[string]any{
			{"key": "KC_DB_PASSWORD", "value": nil},
			{"key": "KC_HOSTNAME", "value": "id.example.com"},
		},
	}), sessionCookie)
	if response.Code != http.StatusOK {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
	if content, _ := os.ReadFile(filepath.Join(folder, "keycloak.env")); string(content) != "KC_DB=postgres\nKC_HOSTNAME=id.example.com\n" {
		tester.Fatalf("file %q", content)
	}
}

// The second of two editors working from the same version gets 409, and the first save stays.
func TestStaleSaveIsRefused(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	version := openForEdit(tester, handler, sessionCookie)
	firstSave := editorBody(map[string]any{"baseVersion": version, "content": "A=1\n"})
	if response := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content", firstSave, sessionCookie); response.Code != http.StatusOK {
		tester.Fatalf("first save: %d", response.Code)
	}
	secondSave := editorBody(map[string]any{"baseVersion": version, "changes": []map[string]any{{"key": "B", "value": "2"}}})
	response := sendJSON(handler, http.MethodPatch, "/api/env-files/keycloak.env/variables", secondSave, sessionCookie)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "reload") {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
}

func TestEditorErrors(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	folder := envFolderWith(tester, handler, sessionCookie)
	version := openForEdit(tester, handler, sessionCookie)
	testCases := []struct {
		method, target, body string
		expectedCode         int
	}{
		{http.MethodPatch, "/api/env-files/keycloak.env/variables", editorBody(map[string]any{"baseVersion": version, "changes": []map[string]any{{"key": "1BAD", "value": "x"}}}), http.StatusBadRequest},
		{http.MethodPatch, "/api/env-files/keycloak.env/variables", editorBody(map[string]any{"baseVersion": version, "changes": []map[string]any{{"key": "A", "value": "x\nB=1"}}}), http.StatusBadRequest},
		{http.MethodPatch, "/api/env-files/keycloak.env/variables", editorBody(map[string]any{"baseVersion": version, "changes": []map[string]any{{"key": "MISSING", "value": nil}}}), http.StatusNotFound},
		{http.MethodPatch, "/api/env-files/keycloak.env/variables", "not json", http.StatusBadRequest},
		{http.MethodPut, "/api/env-files/keycloak.env/content", "not json", http.StatusBadRequest},
		{http.MethodPut, "/api/env-files/missing.env/content", editorBody(map[string]any{"baseVersion": version}), http.StatusNotFound},
		{http.MethodGet, "/api/env-files/missing.env/content", "", http.StatusNotFound},
	}
	for _, testCase := range testCases {
		if response := sendJSON(handler, testCase.method, testCase.target, testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s %s %s: got %d %s", testCase.method, testCase.target, testCase.body, response.Code, response.Body.String())
		}
	}

	// A folder Docu-UI may not write (wrong owner on the host): the reason must reach the user.
	os.Chmod(folder, 0o500)
	tester.Cleanup(func() { os.Chmod(folder, 0o700) })
	response := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content",
		editorBody(map[string]any{"baseVersion": version, "content": "A=1\n"}), sessionCookie)
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "permission denied") {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
}
