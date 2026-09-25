package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/m0od/docu-ui/internal/store"
)

func (failing failingStore) DocoCD(ctx context.Context) (store.DocoCD, error) {
	if failing.failingMethod == "DocoCD" {
		return store.DocoCD{}, errStoreDown
	}
	return failing.Store.DocoCD(ctx)
}

func (failing failingStore) SetDocoCD(ctx context.Context, settings store.DocoCD) error {
	if failing.failingMethod == "SetDocoCD" {
		return errStoreDown
	}
	return failing.Store.SetDocoCD(ctx, settings)
}

func (failing failingStore) ApplyTarget(ctx context.Context, fileName string) (store.ApplyTarget, error) {
	if failing.failingMethod == "ApplyTarget" {
		return store.ApplyTarget{}, errStoreDown
	}
	return failing.Store.ApplyTarget(ctx, fileName)
}

func (failing failingStore) SetApplyTarget(ctx context.Context, fileName string, target store.ApplyTarget) error {
	if failing.failingMethod == "SetApplyTarget" {
		return errStoreDown
	}
	return failing.Store.SetApplyTarget(ctx, fileName, target)
}

// fakeDocoCD answers every recreate with statusCode and records the requested services.
func fakeDocoCD(tester *testing.T, statusCode int) (*httptest.Server, *[]string) {
	tester.Helper()
	recreated := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*recreated = append(*recreated, request.URL.Path+"?"+request.URL.RawQuery+" key="+request.Header.Get("x-api-key"))
		writer.WriteHeader(statusCode)
		_, _ = writer.Write([]byte(`{"error":"project not found"}`))
	}))
	tester.Cleanup(server.Close)
	return server, recreated
}

// readyToApply sets up keycloak.env, Doco-CD at docoCDURL and a target, and returns the file version.
func readyToApply(tester *testing.T, handler http.Handler, sessionCookie *http.Cookie, docoCDURL string, services []string) string {
	tester.Helper()
	envFolderWith(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPut, "/api/settings/doco-cd", editorBody(map[string]any{"url": docoCDURL, "apiKey": "api-key"}), sessionCookie)
	response := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/apply-target",
		editorBody(map[string]any{"project": "textiq-dev", "services": services}), sessionCookie)
	if response.Code != http.StatusOK {
		tester.Fatalf("target: %d %s", response.Code, response.Body.String())
	}
	return openForEdit(tester, handler, sessionCookie)
}

func TestDocoCDSettingsHideTheKey(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	fresh := sendJSON(handler, http.MethodGet, "/api/settings/doco-cd", "", sessionCookie)
	if responseField(tester, fresh, "url") != "" || responseField(tester, fresh, "hasApiKey") != false {
		tester.Fatalf("fresh: %s", fresh.Body.String())
	}

	saved := sendJSON(handler, http.MethodPut, "/api/settings/doco-cd",
		editorBody(map[string]any{"url": "http://doco-cd:80", "apiKey": "api-key"}), sessionCookie)
	if saved.Code != http.StatusOK || strings.Contains(saved.Body.String(), "api-key\"") || responseField(tester, saved, "hasApiKey") != true {
		tester.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	// The page cannot show the key, so saving with an empty key field must keep it.
	sendJSON(handler, http.MethodPut, "/api/settings/doco-cd", editorBody(map[string]any{"url": "http://other:80"}), sessionCookie)
	read := sendJSON(handler, http.MethodGet, "/api/settings/doco-cd", "", sessionCookie)
	if responseField(tester, read, "url") != "http://other:80" || responseField(tester, read, "hasApiKey") != true ||
		strings.Contains(read.Body.String(), "api-key\"") {
		tester.Fatalf("read: %s", read.Body.String())
	}
}

func TestDocoCDSettingsRejectBadInput(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	for _, body := range []string{"not json", `{"url":"doco-cd:80"}`, `{"url":"ftp://doco-cd"}`, `{"url":"http://"}`, `{"url":"http://a b"}`} {
		if response := sendJSON(handler, http.MethodPut, "/api/settings/doco-cd", body, sessionCookie); response.Code != http.StatusBadRequest {
			tester.Errorf("%s: got %d", body, response.Code)
		}
	}
}

// The full loop: set up, save a change, see it is not applied, apply it, see it is.
func TestApplyRecreatesEachService(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	docoCD, recreated := fakeDocoCD(tester, http.StatusOK)
	firstVersion := readyToApply(tester, handler, sessionCookie, docoCD.URL, []string{"keycloak", "iam"})
	// Set up on a running system: the file as it is now counts as applied.
	target := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/apply-target", "", sessionCookie)
	if responseField(tester, target, "appliedVersion") != firstVersion {
		tester.Fatalf("target: %s", target.Body.String())
	}

	saved := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content",
		editorBody(map[string]any{"baseVersion": firstVersion, "content": "KC_DB=mariadb\n"}), sessionCookie)
	newVersion := responseField(tester, saved, "version")
	applied := sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", editorBody(map[string]any{"version": newVersion}), sessionCookie)
	if applied.Code != http.StatusOK || responseField(tester, applied, "appliedVersion") != newVersion {
		tester.Fatalf("apply: %d %s", applied.Code, applied.Body.String())
	}
	expected := []string{
		"/v1/api/project/textiq-dev/recreate?service=keycloak key=api-key",
		"/v1/api/project/textiq-dev/recreate?service=iam key=api-key",
	}
	if strings.Join(*recreated, "\n") != strings.Join(expected, "\n") {
		tester.Fatalf("recreated %v", *recreated)
	}

	// Changing the services later keeps the applied version; nothing was recreated by that.
	changed := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/apply-target",
		editorBody(map[string]any{"project": "textiq-dev", "services": []string{}}), sessionCookie)
	if responseField(tester, changed, "appliedVersion") != newVersion {
		tester.Fatalf("changed: %s", changed.Body.String())
	}
}

func TestApplyWithoutServicesRecreatesTheProject(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	docoCD, recreated := fakeDocoCD(tester, http.StatusOK)
	version := readyToApply(tester, handler, sessionCookie, docoCD.URL, nil)
	sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", editorBody(map[string]any{"version": version}), sessionCookie)
	if len(*recreated) != 1 || (*recreated)[0] != "/v1/api/project/textiq-dev/recreate? key=api-key" {
		tester.Fatalf("recreated %v", *recreated)
	}
}

// A failed recreate must leave the file marked "not applied" and show Doco-CD's reason.
func TestApplyReportsDocoCDFailure(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	docoCD, _ := fakeDocoCD(tester, http.StatusNotFound)
	version := readyToApply(tester, handler, sessionCookie, docoCD.URL, []string{"keycloak"})
	sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/content",
		editorBody(map[string]any{"baseVersion": version, "content": "A=1\n"}), sessionCookie)
	newVersion := openForEdit(tester, handler, sessionCookie)

	response := sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", editorBody(map[string]any{"version": newVersion}), sessionCookie)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "project not found") {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
	target := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/apply-target", "", sessionCookie)
	if responseField(tester, target, "appliedVersion") != version {
		tester.Fatalf("target: %s", target.Body.String())
	}
}

func TestApplyNeedsSetup(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	envFolderWith(tester, handler, sessionCookie)
	version := openForEdit(tester, handler, sessionCookie)
	applyBody := editorBody(map[string]any{"version": version})

	if target := sendJSON(handler, http.MethodGet, "/api/env-files/keycloak.env/apply-target", "", sessionCookie); !strings.Contains(target.Body.String(), `"target":null`) {
		tester.Fatalf("target: %s", target.Body.String())
	}
	response := sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody, sessionCookie)
	if response.Code != http.StatusConflict || responseField(tester, response, "error") != targetNotSet {
		tester.Fatalf("no target: %d %s", response.Code, response.Body.String())
	}
	sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/apply-target", editorBody(map[string]any{"project": "textiq-dev"}), sessionCookie)
	response = sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody, sessionCookie)
	if response.Code != http.StatusConflict || responseField(tester, response, "error") != docoCDNotSet {
		tester.Fatalf("no doco-cd: %d %s", response.Code, response.Body.String())
	}
}

func TestApplyErrors(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	docoCD, recreated := fakeDocoCD(tester, http.StatusOK)
	readyToApply(tester, handler, sessionCookie, docoCD.URL, nil)
	testCases := []struct {
		method, target, body string
		expectedCode         int
	}{
		{http.MethodPut, "/api/env-files/keycloak.env/apply-target", "not json", http.StatusBadRequest},
		{http.MethodPut, "/api/env-files/keycloak.env/apply-target", `{"project":"TextIQ"}`, http.StatusBadRequest},
		{http.MethodPut, "/api/env-files/keycloak.env/apply-target", `{"project":"textiq-dev","services":["a/b"]}`, http.StatusBadRequest},
		{http.MethodPut, "/api/env-files/missing.env/apply-target", `{"project":"textiq-dev"}`, http.StatusNotFound},
		{http.MethodPost, "/api/env-files/keycloak.env/apply", "not json", http.StatusBadRequest},
		{http.MethodPost, "/api/env-files/missing.env/apply", `{"version":"x"}`, http.StatusNotFound},
		// The file changed after the page loaded: the user must see that change before it goes live.
		{http.MethodPost, "/api/env-files/keycloak.env/apply", `{"version":"stale"}`, http.StatusConflict},
	}
	for _, testCase := range testCases {
		if response := sendJSON(handler, testCase.method, testCase.target, testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s %s %s: got %d %s", testCase.method, testCase.target, testCase.body, response.Code, response.Body.String())
		}
	}
	if len(*recreated) != 0 {
		tester.Fatalf("nothing may be recreated on a refused request: %v", *recreated)
	}
}

func TestApplyRoutesReportStoreFailures(tester *testing.T) {
	adminStore := openAdminStore(tester, "")
	docoCD, _ := fakeDocoCD(tester, http.StatusOK)
	handler, sessionCookie := signedIn(tester, adminStore)
	version := readyToApply(tester, handler, sessionCookie, docoCD.URL, nil)
	applyBody := editorBody(map[string]any{"version": version})
	targetBody := editorBody(map[string]any{"project": "textiq-dev"})
	settingsBody := editorBody(map[string]any{"url": "http://doco-cd"})
	testCases := []struct {
		failingMethod, method, target, body string
	}{
		{"DocoCD", http.MethodGet, "/api/settings/doco-cd", ""},
		{"DocoCD", http.MethodPut, "/api/settings/doco-cd", settingsBody},
		{"SetDocoCD", http.MethodPut, "/api/settings/doco-cd", settingsBody},
		{"ApplyTarget", http.MethodGet, "/api/env-files/keycloak.env/apply-target", ""},
		{"ApplyTarget", http.MethodPut, "/api/env-files/keycloak.env/apply-target", targetBody},
		{"SetApplyTarget", http.MethodPut, "/api/env-files/keycloak.env/apply-target", targetBody},
		{"ApplyTarget", http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody},
		{"DocoCD", http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody},
		// Recreated, but not recorded: the page must not claim success.
		{"SetApplyTarget", http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody},
	}
	for _, testCase := range testCases {
		failingHandler, failingCookie := signedIn(tester, failingStore{Store: adminStore, failingMethod: testCase.failingMethod})
		if response := sendJSON(failingHandler, testCase.method, testCase.target, testCase.body, failingCookie); response.Code != http.StatusInternalServerError {
			tester.Errorf("%s %s with %s down: got %d", testCase.method, testCase.target, testCase.failingMethod, response.Code)
		}
	}
}

// Apply-target and apply need the env folder like every other file route.
func TestApplyRoutesNeedTheFolder(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	for _, route := range [][2]string{
		{"/api/env-files/a.env/apply-target", `{"project":"textiq-dev"}`},
		{"/api/env-files/a.env/apply", `{"version":"x"}`},
	} {
		method := http.MethodPut
		if strings.HasSuffix(route[0], "/apply") {
			method = http.MethodPost
		}
		if response := sendJSON(handler, method, route[0], route[1], sessionCookie); response.Code != http.StatusConflict {
			tester.Errorf("%s: got %d", route[0], response.Code)
		}
	}
}
