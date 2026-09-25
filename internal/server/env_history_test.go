package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// historyEntryIDs lists the entries as the history page gets them, newest first.
func historyEntryIDs(tester *testing.T, handler http.Handler, sessionCookie *http.Cookie) []string {
	tester.Helper()
	response := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/history", "", sessionCookie)
	entries, _ := responseField(tester, response, "entries").([]any)
	if response.Code != http.StatusOK || entries == nil {
		tester.Fatalf("history: %d %s", response.Code, response.Body.String())
	}
	entryIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryIDs = append(entryIDs, entry.(map[string]any)["id"].(string))
	}
	return entryIDs
}

func TestHistoryViewAndRestore(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	folder := envFolderWith(tester, handler, sessionCookie)
	// A file never saved here has no history yet: an empty list, not an error.
	if entryIDs := historyEntryIDs(tester, handler, sessionCookie); len(entryIDs) != 0 {
		tester.Fatalf("got %v", entryIDs)
	}

	version := openForEdit(tester, handler, sessionCookie)
	saved := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content",
		editorBody(map[string]any{"baseVersion": version, "content": "KC_DB=mariadb\n"}), sessionCookie)
	entryIDs := historyEntryIDs(tester, handler, sessionCookie)
	if len(entryIDs) != 1 {
		tester.Fatalf("got %v", entryIDs)
	}

	// The old version shows every value, like the text editor, so it must not be cached either.
	opened := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/history/"+entryIDs[0], "", sessionCookie)
	if opened.Code != http.StatusOK || responseField(tester, opened, "content") != "KC_DB=postgres\nKC_DB_PASSWORD=s3cret\n" {
		tester.Fatalf("open: %d %s", opened.Code, opened.Body.String())
	}
	if opened.Header().Get("Cache-Control") != "no-store" {
		tester.Fatal("history content may be cached")
	}

	restored := sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/history/"+entryIDs[0]+"/restore",
		editorBody(map[string]any{"baseVersion": responseField(tester, saved, "version")}), sessionCookie)
	if restored.Code != http.StatusOK {
		tester.Fatalf("restore: %d %s", restored.Code, restored.Body.String())
	}
	if content, _ := os.ReadFile(filepath.Join(folder, "keycloak.env")); string(content) != "KC_DB=postgres\nKC_DB_PASSWORD=s3cret\n" {
		tester.Fatalf("file %q", content)
	}
	// Restoring is a save too: the version it replaced can be restored in turn.
	if entryIDs := historyEntryIDs(tester, handler, sessionCookie); len(entryIDs) != 2 {
		tester.Fatalf("got %v", entryIDs)
	}
}

func TestHistoryErrors(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	version := openForEdit(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content",
		editorBody(map[string]any{"baseVersion": version, "content": "A=1\n"}), sessionCookie)
	entryID := historyEntryIDs(tester, handler, sessionCookie)[0]
	missingID := "20200101T000000.000000000Z_admin.env"

	testCases := []struct {
		method, target, body string
		expectedCode         int
	}{
		{http.MethodGet, "/api/env-files/bad%20name/history", "", http.StatusBadRequest},
		{http.MethodGet, "/api/env-files/keycloak.env/history/" + missingID, "", http.StatusNotFound},
		{http.MethodGet, "/api/env-files/keycloak.env/history/keycloak.env", "", http.StatusNotFound},
		{http.MethodPost, "/api/env-files/keycloak.env/history/" + entryID + "/restore", "not json", http.StatusBadRequest},
		{http.MethodPost, "/api/env-files/keycloak.env/history/" + missingID + "/restore", editorBody(map[string]any{"baseVersion": version}), http.StatusNotFound},
		// version is the one before the save above: someone changed the file since the page loaded.
		{http.MethodPost, "/api/env-files/keycloak.env/history/" + entryID + "/restore", editorBody(map[string]any{"baseVersion": version}), http.StatusConflict},
	}
	for _, testCase := range testCases {
		if response := sendJSON(handler, testCase.method, testCase.target, testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s %s %s: got %d %s", testCase.method, testCase.target, testCase.body, response.Code, response.Body.String())
		}
	}
}
