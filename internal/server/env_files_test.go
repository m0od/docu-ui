package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (failing failingStore) EnvFolder(ctx context.Context) (string, error) {
	if failing.failingMethod == "EnvFolder" {
		return "", errStoreDown
	}
	return failing.Store.EnvFolder(ctx)
}

func (failing failingStore) SetEnvFolder(ctx context.Context, folder string) error {
	if failing.failingMethod == "SetEnvFolder" {
		return errStoreDown
	}
	return failing.Store.SetEnvFolder(ctx, folder)
}

// signedIn returns a server backed by testStore and a session cookie for the admin.
func signedIn(tester *testing.T, testStore Store) (http.Handler, *http.Cookie) {
	tester.Helper()
	handler := newAuthServer(tester, Config{Store: testStore})
	signIn := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, ""))
	return handler, sessionCookieFrom(tester, signIn)
}

func envFolderBody(folder string) string {
	encodedBody, _ := json.Marshal(map[string]string{"folder": folder})
	return string(encodedBody)
}

// envFolderWith creates a folder holding keycloak.env and saves it as the env folder.
func envFolderWith(tester *testing.T, handler http.Handler, sessionCookie *http.Cookie) string {
	tester.Helper()
	folder := tester.TempDir()
	os.WriteFile(filepath.Join(folder, "keycloak.env"), []byte("KC_DB=postgres\nKC_DB_PASSWORD=s3cret\n"), 0o600)
	if saved := sendJSON(handler, http.MethodPut, "/api/settings/env-folder", envFolderBody(folder), sessionCookie); saved.Code != http.StatusOK {
		tester.Fatalf("save folder: %d %s", saved.Code, saved.Body.String())
	}
	return folder
}

// Env files hold production secrets: every route must refuse a visitor without a session.
func TestEnvRoutesRequireSession(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	routes := [][2]string{
		{http.MethodGet, "/api/settings/env-folder"},
		{http.MethodPut, "/api/settings/env-folder"},
		{http.MethodGet, "/api/env-files"},
		{http.MethodGet, "/api/env-files/keycloak.env"},
		{http.MethodGet, "/api/env-files/keycloak.env/variables/KC_DB"},
		{http.MethodPatch, "/api/env-files/keycloak.env/variables"},
		{http.MethodGet, "/api/env-files/keycloak.env/content"},
		{http.MethodPut, "/api/env-files/keycloak.env/content"},
		{http.MethodGet, "/api/env-files/keycloak.env/history"},
		{http.MethodGet, "/api/env-files/keycloak.env/history/20260925T080000.000000000Z_admin.env"},
		{http.MethodPost, "/api/env-files/keycloak.env/history/20260925T080000.000000000Z_admin.env/restore"},
	}
	for _, route := range routes {
		if response := sendJSON(handler, route[0], route[1], envFolderBody("/tmp")); response.Code != http.StatusUnauthorized {
			tester.Errorf("%s %s: got %d", route[0], route[1], response.Code)
		}
	}
}

// A fresh install has no folder: the UI must ask for one, not show an empty list.
func TestEnvFilesAskForFolderFirst(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	if folder := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", "", sessionCookie); responseField(tester, folder, "folder") != "" {
		tester.Fatalf("folder: %s", folder.Body.String())
	}
	routes := [][2]string{
		{http.MethodGet, "/api/env-files"},
		{http.MethodGet, "/api/env-files/a.env"},
		{http.MethodGet, "/api/env-files/a.env/variables/KEY"},
		{http.MethodPatch, "/api/env-files/a.env/variables"},
		{http.MethodGet, "/api/env-files/a.env/content"},
		{http.MethodPut, "/api/env-files/a.env/content"},
		{http.MethodGet, "/api/env-files/a.env/history"},
		{http.MethodGet, "/api/env-files/a.env/history/20260925T080000.000000000Z_admin.env"},
		{http.MethodPost, "/api/env-files/a.env/history/20260925T080000.000000000Z_admin.env/restore"},
	}
	for _, route := range routes {
		method, target := route[0], route[1]
		response := sendJSON(handler, method, target, "{}", sessionCookie)
		if response.Code != http.StatusConflict || responseField(tester, response, "error") != folderNotSet {
			tester.Errorf("%s: %d %s", target, response.Code, response.Body.String())
		}
	}
}

// Changing the folder on the web takes effect on the next request, without a restart.
func TestChangingFolderAppliesImmediately(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	firstFolder := envFolderWith(tester, handler, sessionCookie)
	listing := sendJSON(handler, http.MethodGet, "/api/env-files", "", sessionCookie)
	if listing.Code != http.StatusOK || !strings.Contains(listing.Body.String(), `"files":["keycloak.env"]`) ||
		responseField(tester, listing, "folder") != firstFolder {
		tester.Fatalf("first folder: %d %s", listing.Code, listing.Body.String())
	}

	secondFolder := tester.TempDir()
	os.WriteFile(filepath.Join(secondFolder, "airflow.env"), []byte("A=1\n"), 0o600)
	sendJSON(handler, http.MethodPut, "/api/settings/env-folder", envFolderBody(secondFolder), sessionCookie)
	listing = sendJSON(handler, http.MethodGet, "/api/env-files", "", sessionCookie)
	if !strings.Contains(listing.Body.String(), `"files":["airflow.env"]`) {
		tester.Fatalf("second folder: %s", listing.Body.String())
	}
	if saved := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", "", sessionCookie); responseField(tester, saved, "folder") != secondFolder {
		tester.Fatalf("saved folder: %s", saved.Body.String())
	}
}

// A typo or an unmounted path must not replace a folder that works.
func TestInvalidFolderKeepsTheOldOne(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	workingFolder := envFolderWith(tester, handler, sessionCookie)
	for _, badFolder := range []string{"relative/env", filepath.Join(workingFolder, "missing")} {
		response := sendJSON(handler, http.MethodPut, "/api/settings/env-folder", envFolderBody(badFolder), sessionCookie)
		if response.Code != http.StatusBadRequest {
			tester.Errorf("%q: got %d", badFolder, response.Code)
		}
	}
	if saved := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", "", sessionCookie); responseField(tester, saved, "folder") != workingFolder {
		tester.Fatalf("folder replaced: %s", saved.Body.String())
	}
}

func TestSetFolderRequiresJSON(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	request := httptest.NewRequest(http.MethodPut, "/api/settings/env-folder", strings.NewReader("folder=/tmp"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(sessionCookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnsupportedMediaType {
		tester.Fatalf("got %d", recorder.Code)
	}
}

// The file view lists keys only; a value leaves the server only when asked for one by one.
func TestReadFileNeverSendsValues(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	response := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env", "", sessionCookie)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"key":"KC_DB_PASSWORD"`) {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "s3cret") {
		tester.Fatal("value leaked in the file view")
	}
}

func TestRevealValue(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	response := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/variables/KC_DB_PASSWORD", "", sessionCookie)
	if response.Code != http.StatusOK || responseField(tester, response, "value") != "s3cret" {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
	// A cached secret would outlive sign-out in the browser or a proxy.
	if response.Header().Get("Cache-Control") != "no-store" {
		tester.Fatal("secret response may be cached")
	}
}

func TestEnvFileErrors(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	folder := envFolderWith(tester, handler, sessionCookie)
	os.Symlink("/etc/hosts", filepath.Join(folder, "escape.env"))
	expectedCodes := map[string]int{
		"/api/env-files/..%2Fsecret.env":                 http.StatusBadRequest,
		"/api/env-files/missing.env":                     http.StatusNotFound,
		"/api/env-files/keycloak.env/variables/MISSING":  http.StatusNotFound,
		"/api/env-files/escape.env":                      http.StatusInternalServerError,
		"/api/env-files/escape.env/variables/KEY":        http.StatusInternalServerError,
		"/api/env-files/..%2Fsecret.env/variables/KC_DB": http.StatusBadRequest,
	}
	for target, expectedCode := range expectedCodes {
		if response := sendJSON(handler, http.MethodGet, target, "", sessionCookie); response.Code != expectedCode {
			tester.Errorf("%s: got %d %s", target, response.Code, response.Body.String())
		}
	}
	// The mounted volume disappears after the folder was saved.
	os.RemoveAll(folder)
	listing := sendJSON(handler, http.MethodGet, "/api/env-files", "", sessionCookie)
	if listing.Code != http.StatusInternalServerError || !strings.Contains(listing.Body.String(), "cannot read the env folder") {
		tester.Fatalf("got %d %s", listing.Code, listing.Body.String())
	}
}

func TestEnvRoutesReportStoreFailures(tester *testing.T) {
	adminStore := openAdminStore(tester, "")
	handler, sessionCookie := signedIn(tester, failingStore{Store: adminStore, failingMethod: "EnvFolder"})
	for _, target := range []string{"/api/settings/env-folder", "/api/env-files"} {
		if response := sendJSON(handler, http.MethodGet, target, "", sessionCookie); response.Code != http.StatusInternalServerError {
			tester.Errorf("%s: got %d", target, response.Code)
		}
	}
	handler, sessionCookie = signedIn(tester, failingStore{Store: adminStore, failingMethod: "SetEnvFolder"})
	response := sendJSON(handler, http.MethodPut, "/api/settings/env-folder", envFolderBody(tester.TempDir()), sessionCookie)
	if response.Code != http.StatusInternalServerError {
		tester.Fatalf("got %d", response.Code)
	}
}
