package server

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
)

const newPassword = "a-new-long-password"

// signedInWithTOTP signs in to an account that has authTestSecret, at unix time 59 (code 287082 is then used).
func signedInWithTOTP(tester *testing.T, testStore Store) (http.Handler, *http.Cookie) {
	tester.Helper()
	setClock(tester, time.Unix(59, 0))
	handler := newAuthServer(tester, Config{Store: testStore})
	return handler, sessionCookieFrom(tester, sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "287082")))
}

func signInCode(handler http.Handler, password, totpCode string) int {
	return sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", password, totpCode)).Code
}

func TestAccountSaysWhetherTOTPIsOn(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	account := sendJSON(handler, http.MethodGet, "/api/account", "", sessionCookie)
	if account.Code != http.StatusOK || responseField(tester, account, "username") != "admin" || responseField(tester, account, "totpEnabled") != false {
		tester.Fatalf("got %d %s", account.Code, account.Body.String())
	}
	handler, sessionCookie = signedInWithTOTP(tester, openAdminStore(tester, authTestSecret))
	if account := sendJSON(handler, http.MethodGet, "/api/account", "", sessionCookie); responseField(tester, account, "totpEnabled") != true {
		tester.Fatalf("got %s", account.Body.String())
	}
}

// The old password stops working, and so does every other session: whoever knew it is locked out.
func TestChangePasswordSignsOutOtherSessions(tester *testing.T) {
	testStore := openAdminStore(tester, "")
	handler, sessionCookie := signedIn(tester, testStore)
	_, otherDeviceCookie := signedIn(tester, testStore)

	changed := sendJSON(handler, http.MethodPut, "/api/account/password", editorBody(map[string]any{"currentPassword": adminPassword, "newPassword": newPassword}), sessionCookie)
	if changed.Code != http.StatusNoContent {
		tester.Fatalf("change: %d %s", changed.Code, changed.Body.String())
	}
	if me := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie); me.Code != http.StatusOK {
		tester.Fatalf("the session that changed it must go on: %d", me.Code)
	}
	if me := sendJSON(handler, http.MethodGet, "/api/auth/me", "", otherDeviceCookie); me.Code != http.StatusUnauthorized {
		tester.Fatalf("other session: %d", me.Code)
	}
	if signInCode(handler, adminPassword, "") != http.StatusUnauthorized || signInCode(handler, newPassword, "") != http.StatusOK {
		tester.Fatal("only the new password may sign in")
	}
}

func TestChangePasswordRejectsBadInput(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	testCases := map[string]struct {
		body         string
		expectedCode int
	}{
		"not json":       {"not json", http.StatusBadRequest},
		"short password": {editorBody(map[string]any{"currentPassword": adminPassword, "newPassword": "short"}), http.StatusBadRequest},
		// 403, not 401: a 401 would send the UI back to sign-in although the session is fine.
		"wrong current": {editorBody(map[string]any{"currentPassword": "wrong-password", "newPassword": newPassword}), http.StatusForbidden},
	}
	for name, testCase := range testCases {
		if response := sendJSON(handler, http.MethodPut, "/api/account/password", testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s: got %d %s", name, response.Code, response.Body.String())
		}
	}
	if signInCode(handler, adminPassword, "") != http.StatusOK {
		tester.Fatal("a rejected change must keep the old password")
	}
}

// Guessing the current password from an open session counts towards the sign-in lockout.
func TestWrongCurrentPasswordsLockTheAccount(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	wrongBody := editorBody(map[string]any{"currentPassword": "wrong-password", "newPassword": newPassword})
	for attempt := 1; attempt <= maxFailedLogins; attempt++ {
		sendJSON(handler, http.MethodPut, "/api/account/password", wrongBody, sessionCookie)
	}
	rightBody := editorBody(map[string]any{"currentPassword": adminPassword, "newPassword": newPassword})
	if locked := sendJSON(handler, http.MethodPut, "/api/account/password", rightBody, sessionCookie); locked.Code != http.StatusTooManyRequests {
		tester.Fatalf("got %d %s", locked.Code, locked.Body.String())
	}
}

// Turning TOTP on needs a working code; afterwards sign-in asks for it, and the enabling code is spent.
func TestEnableTOTP(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	secretResponse := sendJSON(handler, http.MethodGet, "/api/account/totp-secret", "", sessionCookie)
	if secret, _ := responseField(tester, secretResponse, "secret").(string); !auth.ValidTOTPSecret(secret) {
		tester.Fatalf("secret: %s", secretResponse.Body.String())
	}
	setClock(tester, time.Unix(59, 0))
	enableBody := func(password, secret, code string) string {
		return editorBody(map[string]any{"currentPassword": password, "secret": secret, "code": code})
	}
	rejected := map[string]struct {
		body         string
		expectedCode int
	}{
		"not json":       {"not json", http.StatusBadRequest},
		"invalid secret": {enableBody(adminPassword, "not-base32!", "287082"), http.StatusBadRequest},
		"wrong password": {enableBody("wrong-password", authTestSecret, "287082"), http.StatusForbidden},
		"wrong code":     {enableBody(adminPassword, authTestSecret, "000000"), http.StatusBadRequest},
	}
	for name, testCase := range rejected {
		if response := sendJSON(handler, http.MethodPost, "/api/account/totp", testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s: got %d %s", name, response.Code, response.Body.String())
		}
	}

	if enabled := sendJSON(handler, http.MethodPost, "/api/account/totp", enableBody(adminPassword, authTestSecret, "287082"), sessionCookie); enabled.Code != http.StatusNoContent {
		tester.Fatalf("enable: %d %s", enabled.Code, enabled.Body.String())
	}
	if again := sendJSON(handler, http.MethodPost, "/api/account/totp", enableBody(adminPassword, authTestSecret, "287082"), sessionCookie); again.Code != http.StatusConflict {
		tester.Fatalf("enable twice: %d", again.Code)
	}
	if signInCode(handler, adminPassword, "") != http.StatusUnauthorized {
		tester.Fatal("sign-in must ask for the code now")
	}
	if replayed := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "287082")); !strings.Contains(replayed.Body.String(), "already used") {
		tester.Fatalf("the enabling code must not sign in: %s", replayed.Body.String())
	}
}

// Turning TOTP off needs both factors, like signing in; a code already used does not count.
func TestDisableTOTP(tester *testing.T) {
	handler, sessionCookie := signedInWithTOTP(tester, openAdminStore(tester, authTestSecret))
	disableBody := func(password, code string) string {
		return editorBody(map[string]any{"currentPassword": password, "code": code})
	}
	rejected := map[string]struct {
		body         string
		expectedCode int
	}{
		"not json":       {"not json", http.StatusBadRequest},
		"wrong password": {disableBody("wrong-password", "287082"), http.StatusForbidden},
		"wrong code":     {disableBody(adminPassword, "000000"), http.StatusForbidden},
		// 287082 signed in above; seeing it once must not be enough to turn TOTP off.
		"used code": {disableBody(adminPassword, "287082"), http.StatusForbidden},
	}
	for name, testCase := range rejected {
		if response := sendJSON(handler, http.MethodPost, "/api/account/totp/disable", testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s: got %d %s", name, response.Code, response.Body.String())
		}
	}

	// At unix time 119 (step 3) the code is 969429; the session is still fresh.
	setClock(tester, time.Unix(119, 0))
	if disabled := sendJSON(handler, http.MethodPost, "/api/account/totp/disable", disableBody(adminPassword, "969429"), sessionCookie); disabled.Code != http.StatusNoContent {
		tester.Fatalf("disable: %d %s", disabled.Code, disabled.Body.String())
	}
	if signInCode(handler, adminPassword, "") != http.StatusOK {
		tester.Fatal("sign-in must not ask for a code any more")
	}
	if again := sendJSON(handler, http.MethodPost, "/api/account/totp/disable", disableBody(adminPassword, "969429"), sessionCookie); again.Code != http.StatusConflict {
		tester.Fatalf("disable twice: %d", again.Code)
	}
}

func TestAccountReportsStoreFailures(tester *testing.T) {
	passwordBody := editorBody(map[string]any{"currentPassword": adminPassword, "newPassword": newPassword})
	wrongPasswordBody := editorBody(map[string]any{"currentPassword": "wrong-password", "newPassword": newPassword})
	enableBody := editorBody(map[string]any{"currentPassword": adminPassword, "secret": authTestSecret, "code": "287082"})
	// 969429 is valid at unix 119, set below; 287082 was spent on sign-in.
	disableBody := editorBody(map[string]any{"currentPassword": adminPassword, "code": "969429"})
	failingCases := []struct {
		failingMethod, totpSecret, method, target, body string
	}{
		{"FindAccount", "", http.MethodGet, "/api/account", ""},
		{"FindAccount", "", http.MethodPut, "/api/account/password", passwordBody},
		{"RecordFailedLogin", "", http.MethodPut, "/api/account/password", wrongPasswordBody},
		{"SetPassword", "", http.MethodPut, "/api/account/password", passwordBody},
		{"DeleteOtherSessions", "", http.MethodPut, "/api/account/password", passwordBody},
		{"SetTOTP", "", http.MethodPost, "/api/account/totp", enableBody},
		{"UseTOTPStep", authTestSecret, http.MethodPost, "/api/account/totp/disable", disableBody},
		{"SetTOTP", authTestSecret, http.MethodPost, "/api/account/totp/disable", disableBody},
	}
	for _, failingCase := range failingCases {
		testStore := openAdminStore(tester, failingCase.totpSecret)
		// Sign in on the real store; the session lives in the database the failing store shares.
		_, sessionCookie := signedInWithTOTP(tester, testStore)
		if failingCase.totpSecret == "" {
			_, sessionCookie = signedIn(tester, testStore)
		}
		if failingCase.target == "/api/account/totp/disable" {
			setClock(tester, time.Unix(119, 0))
		}
		failingHandler := newAuthServer(tester, Config{Store: failingStore{Store: testStore, failingMethod: failingCase.failingMethod}})
		if response := sendJSON(failingHandler, failingCase.method, failingCase.target, failingCase.body, sessionCookie); response.Code != http.StatusInternalServerError {
			tester.Errorf("%s on %s: got %d %s", failingCase.failingMethod, failingCase.target, response.Code, response.Body.String())
		}
	}
}
