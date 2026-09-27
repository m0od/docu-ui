package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0od/docu-ui/internal/store"
)

func openSignInOffStore(tester *testing.T) *store.Store {
	tester.Helper()
	testStore, err := store.Open(filepath.Join(tester.TempDir(), "docu-ui.db"))
	if err != nil {
		tester.Fatal(err)
	}
	tester.Cleanup(func() { testStore.Close() })
	if err := testStore.SkipSignIn(context.Background()); err != nil {
		tester.Fatal(err)
	}
	return testStore
}

// Skipping sign-in still needs the setup token: otherwise whoever reaches a fresh install first decides.
func TestSetupCanSkipSignIn(tester *testing.T) {
	accounts := &fakeAccounts{}
	handler := newSetupServer(tester, accounts, testSetupToken)
	wrongToken := postSetup(handler, "application/json", `{"setupToken":"guess","skipSignIn":true}`)
	if wrongToken.Code != http.StatusForbidden || accounts.signInSkipped {
		tester.Fatalf("wrong token: %d skipped=%v", wrongToken.Code, accounts.signInSkipped)
	}
	skipped := postSetup(handler, "application/json", `{"setupToken":"`+testSetupToken+`","skipSignIn":true}`)
	if skipped.Code != http.StatusCreated || !accounts.signInSkipped || accounts.createdUsername != "" {
		tester.Fatalf("got %d %s skipped=%v", skipped.Code, skipped.Body.String(), accounts.signInSkipped)
	}
}

func TestSkipSignInReportsStoreErrors(tester *testing.T) {
	expectedCodes := map[error]int{store.ErrSetupDone: http.StatusConflict, errStoreDown: http.StatusInternalServerError}
	for writeError, expectedCode := range expectedCodes {
		handler := newSetupServer(tester, &fakeAccounts{writeError: writeError}, testSetupToken)
		if response := postSetup(handler, "application/json", `{"setupToken":"`+testSetupToken+`","skipSignIn":true}`); response.Code != expectedCode {
			tester.Errorf("%v: got %d", writeError, response.Code)
		}
	}
}

// With sign-in off there is no session to check: every route answers, as "anonymous".
func TestSignInOffOpensEveryRoute(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openSignInOffStore(tester)})
	if folder := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", ""); folder.Code != http.StatusOK {
		tester.Fatalf("env folder: %d %s", folder.Code, folder.Body.String())
	}
	me := sendJSON(handler, http.MethodGet, "/api/auth/me", "")
	if responseField(tester, me, "username") != anonymousUser || responseField(tester, me, "signIn") != false {
		tester.Fatalf("me: %s", me.Body.String())
	}
	if account := sendJSON(handler, http.MethodGet, "/api/account", ""); responseField(tester, account, "signIn") != false {
		tester.Fatalf("account: %s", account.Body.String())
	}
}

// The UI shows the no-sign-in banner from this flag, so sign-in mode must say true.
func TestMeSaysSignInIsOn(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	if me := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie); responseField(tester, me, "signIn") != true {
		tester.Fatalf("me: %s", me.Body.String())
	}
	if account := sendJSON(handler, http.MethodGet, "/api/account", "", sessionCookie); responseField(tester, account, "signIn") != true {
		tester.Fatalf("account: %s", account.Body.String())
	}
}

// Failing to read the mode must not fall back to "open".
func TestSessionCheckFailsClosed(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: failingStore{Store: openSignInOffStore(tester), failingMethod: "SignInOff"}})
	if response := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", ""); response.Code != http.StatusInternalServerError {
		tester.Fatalf("got %d", response.Code)
	}
}

// The mode was read fine but the session was not: still no access.
func TestSessionReadFailureFailsClosed(tester *testing.T) {
	testStore := openAdminStore(tester, "")
	_, sessionCookie := signedIn(tester, testStore)
	handler := newAuthServer(tester, Config{Store: failingStore{Store: testStore, failingMethod: "UseSession"}})
	if response := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie); response.Code != http.StatusInternalServerError {
		tester.Fatalf("got %d", response.Code)
	}
}

// Password and TOTP changes need an account; without one they say why instead of failing.
func TestAccountChangesNeedSignIn(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openSignInOffStore(tester)})
	confirmBody := editorBody(map[string]any{"currentPassword": adminPassword, "newPassword": newPassword, "secret": authTestSecret, "code": "287082"})
	for _, target := range []string{"/api/account/totp", "/api/account/totp/disable", "/api/account/sign-in/disable"} {
		if response := sendJSON(handler, http.MethodPost, target, confirmBody); response.Code != http.StatusConflict {
			tester.Errorf("%s: got %d", target, response.Code)
		}
	}
	if response := sendJSON(handler, http.MethodPut, "/api/account/password", confirmBody); response.Code != http.StatusConflict {
		tester.Errorf("password: got %d", response.Code)
	}
}

// Turning sign-in on locks the UI at once: the browser that did it must sign in too.
func TestTurnOnSignIn(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openSignInOffStore(tester)})
	rejected := map[string]string{
		"not json":     "not json",
		"bad username": editorBody(map[string]any{"username": "a", "password": adminPassword}),
		"short":        editorBody(map[string]any{"username": "admin", "password": "short"}),
	}
	for name, body := range rejected {
		if response := sendJSON(handler, http.MethodPost, "/api/account/sign-in", body); response.Code != http.StatusBadRequest {
			tester.Errorf("%s: got %d", name, response.Code)
		}
	}
	turnedOn := sendJSON(handler, http.MethodPost, "/api/account/sign-in", editorBody(map[string]any{"username": "admin", "password": adminPassword}))
	if turnedOn.Code != http.StatusCreated {
		tester.Fatalf("turn on: %d %s", turnedOn.Code, turnedOn.Body.String())
	}
	if folder := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", ""); folder.Code != http.StatusUnauthorized {
		tester.Fatalf("without a session now: %d", folder.Code)
	}
	if signInCode(handler, adminPassword, "") != http.StatusOK {
		tester.Fatal("the new account must sign in")
	}
}

// With sign-in on, nobody (not even a signed-in admin) can add a second account this way.
func TestTurnOnSignInRefusedWhenOn(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	body := editorBody(map[string]any{"username": "second", "password": adminPassword})
	if response := sendJSON(handler, http.MethodPost, "/api/account/sign-in", body, sessionCookie); response.Code != http.StatusConflict {
		tester.Fatalf("got %d", response.Code)
	}
}

func TestTurnOnSignInReportsStoreErrors(tester *testing.T) {
	expectedCodes := map[string]int{"CreateFirstAccountRace": http.StatusConflict, "CreateFirstAccount": http.StatusInternalServerError}
	body := editorBody(map[string]any{"username": "admin", "password": adminPassword})
	for failingMethod, expectedCode := range expectedCodes {
		handler := newAuthServer(tester, Config{Store: failingStore{Store: openSignInOffStore(tester), failingMethod: failingMethod}})
		if response := sendJSON(handler, http.MethodPost, "/api/account/sign-in", body); response.Code != expectedCode {
			tester.Errorf("%s: got %d", failingMethod, response.Code)
		}
	}
}

// Turning sign-in off opens every env file to anyone, so it needs the password.
func TestTurnOffSignIn(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	rejected := map[string]struct {
		body         string
		expectedCode int
	}{
		"not json":       {"not json", http.StatusBadRequest},
		"wrong password": {editorBody(map[string]any{"currentPassword": "wrong-password"}), http.StatusForbidden},
	}
	for name, testCase := range rejected {
		if response := sendJSON(handler, http.MethodPost, "/api/account/sign-in/disable", testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s: got %d", name, response.Code)
		}
	}
	if folder := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", ""); folder.Code != http.StatusUnauthorized {
		tester.Fatalf("a refused request must keep sign-in on: %d", folder.Code)
	}

	turnedOff := sendJSON(handler, http.MethodPost, "/api/account/sign-in/disable", editorBody(map[string]any{"currentPassword": adminPassword}), sessionCookie)
	if turnedOff.Code != http.StatusNoContent {
		tester.Fatalf("turn off: %d %s", turnedOff.Code, turnedOff.Body.String())
	}
	if clearedCookie := turnedOff.Result().Cookies(); len(clearedCookie) != 1 || clearedCookie[0].MaxAge >= 0 {
		tester.Fatalf("the session cookie must be cleared: %v", clearedCookie)
	}
	if folder := sendJSON(handler, http.MethodGet, "/api/settings/env-folder", ""); folder.Code != http.StatusOK {
		tester.Fatalf("open now: %d", folder.Code)
	}
	if signInCode(handler, adminPassword, "") != http.StatusUnauthorized {
		tester.Fatal("the account must be gone")
	}
}

// With TOTP on, the password alone must not be enough, as at sign-in.
func TestTurnOffSignInNeedsTheCode(tester *testing.T) {
	handler, sessionCookie := signedInWithTOTP(tester, openAdminStore(tester, authTestSecret))
	for _, code := range []string{"", "000000", "287082"} {
		response := sendJSON(handler, http.MethodPost, "/api/account/sign-in/disable", editorBody(map[string]any{"currentPassword": adminPassword, "code": code}), sessionCookie)
		if response.Code != http.StatusForbidden {
			tester.Errorf("code %q: got %d", code, response.Code)
		}
	}
	setClock(tester, time.Unix(119, 0))
	if response := sendJSON(handler, http.MethodPost, "/api/account/sign-in/disable", editorBody(map[string]any{"currentPassword": adminPassword, "code": "969429"}), sessionCookie); response.Code != http.StatusNoContent {
		tester.Fatalf("right code: %d %s", response.Code, response.Body.String())
	}
}

func TestTurnOffSignInReportsStoreErrors(tester *testing.T) {
	testStore := openAdminStore(tester, "")
	_, sessionCookie := signedIn(tester, testStore)
	handler := newAuthServer(tester, Config{Store: failingStore{Store: testStore, failingMethod: "TurnOffSignIn"}})
	if response := sendJSON(handler, http.MethodPost, "/api/account/sign-in/disable", editorBody(map[string]any{"currentPassword": adminPassword}), sessionCookie); response.Code != http.StatusInternalServerError {
		tester.Fatalf("got %d", response.Code)
	}
}

// With sign-in off there is no cookie for SameSite to protect, so the JSON content type is all that
// stops any web page the owner visits from posting to Docu-UI: browsers send text/plain, form and
// multipart bodies cross-site without asking, but not application/json. Every write route must refuse
// them, in both modes, before doing anything.
func TestWriteRoutesRefuseCrossSiteContentTypes(tester *testing.T) {
	writeRoutes := [][2]string{
		{http.MethodPost, "/api/setup"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPut, "/api/settings/env-folder"},
		{http.MethodPatch, "/api/env-files/api.env/variables"},
		{http.MethodPut, "/api/env-files/api.env/content"},
		{http.MethodPost, "/api/env-files/api.env/history/20260925T080000.000000000Z_admin.env/restore"},
		{http.MethodPut, "/api/settings/doco-cd"},
		{http.MethodPut, "/api/settings/webhook"},
		{http.MethodPut, "/api/env-files/api.env/apply-target"},
		{http.MethodPost, "/api/env-files/api.env/apply"},
		{http.MethodPut, "/api/account/password"},
		{http.MethodPost, "/api/account/totp"},
		{http.MethodPost, "/api/account/totp/disable"},
		{http.MethodPost, "/api/account/recovery-codes"},
		{http.MethodPost, "/api/account/sign-in"},
		{http.MethodPost, "/api/account/sign-in/disable"},
	}
	crossSiteContentTypes := []string{"text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x", ""}
	// Valid for the route that would hurt most: turning sign-in on with the attacker's password.
	attackBody := `{"username":"intruder","password":"intruder-password","folder":"/"}`

	signInOffStore := openSignInOffStore(tester)
	signInOffHandler := newAuthServer(tester, Config{Store: signInOffStore})
	signedInHandler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	modes := map[string]struct {
		handler http.Handler
		cookies []*http.Cookie
	}{
		"sign-in off": {signInOffHandler, nil},
		"signed in":   {signedInHandler, []*http.Cookie{sessionCookie}},
	}
	for modeName, mode := range modes {
		for _, route := range writeRoutes {
			for _, contentType := range crossSiteContentTypes {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(route[0], route[1], strings.NewReader(attackBody))
				if contentType != "" {
					request.Header.Set("Content-Type", contentType)
				}
				for _, cookie := range mode.cookies {
					request.AddCookie(cookie)
				}
				mode.handler.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusUnsupportedMediaType {
					tester.Errorf("%s, %s %s as %q: got %d %s", modeName, route[0], route[1], contentType, recorder.Code, recorder.Body.String())
				}
			}
		}
	}
	if signInOff, _ := signInOffStore.SignInOff(context.Background()); !signInOff {
		tester.Fatal("a cross-site post turned sign-in on")
	}
	if folder, _ := signInOffStore.EnvFolder(context.Background()); folder != "" {
		tester.Fatalf("a cross-site post set the env folder to %q", folder)
	}
}

// A charset parameter is still JSON: the check must not refuse what the UI and API clients send.
func TestJSONWithCharsetIsAccepted(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openSignInOffStore(tester)})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/settings/env-folder", strings.NewReader(envFolderBody(tester.TempDir())))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		tester.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
}
