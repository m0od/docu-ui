package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

func recoveryCodesFrom(tester *testing.T, recorder *httptest.ResponseRecorder) []string {
	tester.Helper()
	var codesResult struct{ RecoveryCodes []string }
	if err := json.Unmarshal(recorder.Body.Bytes(), &codesResult); err != nil {
		tester.Fatalf("%v: %s", err, recorder.Body.String())
	}
	return codesResult.RecoveryCodes
}

// storeRecoveryCodes gives the admin account these recovery codes, as enabling TOTP would.
func storeRecoveryCodes(tester *testing.T, testStore *store.Store, recoveryCodes ...string) {
	tester.Helper()
	account, err := testStore.FindAccount(context.Background(), "admin")
	if err != nil {
		tester.Fatal(err)
	}
	codeHashes := make([]string, len(recoveryCodes))
	for index, recoveryCode := range recoveryCodes {
		codeHashes[index] = auth.HashRecoveryCode(recoveryCode)
	}
	if err := testStore.ReplaceRecoveryCodes(context.Background(), account.ID, codeHashes); err != nil {
		tester.Fatal(err)
	}
}

func recoveryCodesLeft(tester *testing.T, handler http.Handler, sessionCookie *http.Cookie) any {
	tester.Helper()
	return responseField(tester, sendJSON(handler, http.MethodGet, "/api/account", "", sessionCookie), "recoveryCodesLeft")
}

func confirmBody(password, code string) string {
	return editorBody(map[string]any{"currentPassword": password, "code": code})
}

// A lost phone must not lock the admin out: a recovery code replaces the TOTP code once.
// It is typed by hand, so case and the dash do not matter.
func TestRecoveryCodeSignsInOnce(tester *testing.T) {
	testStore := openAdminStore(tester, authTestSecret)
	storeRecoveryCodes(tester, testStore, "abcde-fghij", "klmno-pqrst")
	handler, sessionCookie := signedInWithTOTP(tester, testStore)
	if left := recoveryCodesLeft(tester, handler, sessionCookie); left != float64(2) {
		tester.Fatalf("left before: %v", left)
	}

	if signInCode(handler, adminPassword, "ABCDEFGHIJ") != http.StatusOK {
		tester.Fatal("a recovery code must sign in")
	}
	reused := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "abcde-fghij"))
	if reused.Code != http.StatusUnauthorized || responseField(tester, reused, "error") != wrongTOTPCode {
		tester.Fatalf("reused code: %d %s", reused.Code, reused.Body.String())
	}
	if left := recoveryCodesLeft(tester, handler, sessionCookie); left != float64(1) {
		tester.Fatalf("left after: %v", left)
	}
	// The other code is untouched, and a wrong password never spends one.
	if signInCode(handler, "wrong-password", "klmno-pqrst") != http.StatusUnauthorized {
		tester.Fatal("wrong password signed in")
	}
	if left := recoveryCodesLeft(tester, handler, sessionCookie); left != float64(1) {
		tester.Fatalf("a wrong password spent a code: %v left", left)
	}
}

// Without TOTP there is nothing for recovery codes to replace, so the count is not shown.
func TestAccountWithoutTOTPHasNoRecoveryCount(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	if left := recoveryCodesLeft(tester, handler, sessionCookie); left != nil {
		tester.Fatalf("got %v", left)
	}
}

// With the phone gone, the admin can still turn TOTP off, since that is the way back in.
func TestRecoveryCodeDisablesTOTP(tester *testing.T) {
	testStore := openAdminStore(tester, authTestSecret)
	storeRecoveryCodes(tester, testStore, "abcde-fghij")
	handler, sessionCookie := signedInWithTOTP(tester, testStore)
	if disabled := sendJSON(handler, http.MethodPost, "/api/account/totp/disable", confirmBody(adminPassword, "abcde-fghij"), sessionCookie); disabled.Code != http.StatusNoContent {
		tester.Fatalf("disable: %d %s", disabled.Code, disabled.Body.String())
	}
}

// New codes replace the old set at once, so a leaked list stops working.
func TestRegenerateRecoveryCodes(tester *testing.T) {
	testStore := openAdminStore(tester, authTestSecret)
	storeRecoveryCodes(tester, testStore, "abcde-fghij", "klmno-pqrst")
	handler, sessionCookie := signedInWithTOTP(tester, testStore)
	rejected := map[string]struct {
		body         string
		expectedCode int
	}{
		"not json":       {"not json", http.StatusBadRequest},
		"wrong password": {confirmBody("wrong-password", "abcde-fghij"), http.StatusForbidden},
		"wrong code":     {confirmBody(adminPassword, "000000"), http.StatusForbidden},
		// 287082 signed in above.
		"used code": {confirmBody(adminPassword, "287082"), http.StatusForbidden},
	}
	for name, testCase := range rejected {
		if response := sendJSON(handler, http.MethodPost, "/api/account/recovery-codes", testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s: got %d %s", name, response.Code, response.Body.String())
		}
	}

	// A recovery code works here too: the admin who lost the phone gets a fresh list.
	regenerated := sendJSON(handler, http.MethodPost, "/api/account/recovery-codes", confirmBody(adminPassword, "abcde-fghij"), sessionCookie)
	newCodes := recoveryCodesFrom(tester, regenerated)
	if regenerated.Code != http.StatusOK || len(newCodes) != auth.RecoveryCodeCount {
		tester.Fatalf("regenerate: %d %s", regenerated.Code, regenerated.Body.String())
	}
	if signInCode(handler, adminPassword, "klmno-pqrst") != http.StatusUnauthorized {
		tester.Fatal("an old code still signs in")
	}
	if signInCode(handler, adminPassword, newCodes[0]) != http.StatusOK {
		tester.Fatal("a new code must sign in")
	}
}

func TestRegenerateRecoveryCodesNeedsTOTP(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	if response := sendJSON(handler, http.MethodPost, "/api/account/recovery-codes", confirmBody(adminPassword, "287082"), sessionCookie); response.Code != http.StatusConflict {
		tester.Fatalf("got %d %s", response.Code, response.Body.String())
	}
}

func TestRecoveryCodesReportStoreFailures(tester *testing.T) {
	failingCases := []struct {
		failingMethod, method, target, body string
	}{
		{"CountRecoveryCodes", http.MethodGet, "/api/account", ""},
		{"UseRecoveryCode", http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "abcde-fghij")},
		{"UseRecoveryCode", http.MethodPost, "/api/account/recovery-codes", confirmBody(adminPassword, "abcde-fghij")},
		{"ReplaceRecoveryCodes", http.MethodPost, "/api/account/recovery-codes", confirmBody(adminPassword, "abcde-fghij")},
	}
	for _, failingCase := range failingCases {
		testStore := openAdminStore(tester, authTestSecret)
		storeRecoveryCodes(tester, testStore, "abcde-fghij")
		_, sessionCookie := signedInWithTOTP(tester, testStore)
		failingHandler := newAuthServer(tester, Config{Store: failingStore{Store: testStore, failingMethod: failingCase.failingMethod}})
		if response := sendJSON(failingHandler, failingCase.method, failingCase.target, failingCase.body, sessionCookie); response.Code != http.StatusInternalServerError {
			tester.Errorf("%s on %s: got %d %s", failingCase.failingMethod, failingCase.target, response.Code, response.Body.String())
		}
	}
	// The account keeps TOTP on even if its first codes cannot be saved; the message says what to do.
	testStore := openAdminStore(tester, "")
	setClock(tester, time.Unix(59, 0))
	handler, sessionCookie := signedIn(tester, failingStore{Store: testStore, failingMethod: "ReplaceRecoveryCodes"})
	enableBody := editorBody(map[string]any{"currentPassword": adminPassword, "secret": authTestSecret, "code": "287082"})
	enabled := sendJSON(handler, http.MethodPost, "/api/account/totp", enableBody, sessionCookie)
	if enabled.Code != http.StatusInternalServerError || !strings.Contains(enabled.Body.String(), "TOTP is on") {
		tester.Fatalf("got %d %s", enabled.Code, enabled.Body.String())
	}
}

// 50 bits per recovery code are out of reach only because wrong codes count toward the lockout too.
// The wrong guesses, and the attempt made while locked, must not spend the real code.
func TestWrongRecoveryCodesLockAccount(tester *testing.T) {
	testStore := openAdminStore(tester, authTestSecret)
	storeRecoveryCodes(tester, testStore, "abcde-fghij")
	handler, sessionCookie := signedInWithTOTP(tester, testStore)
	for attempt := 1; attempt <= maxFailedLogins; attempt++ {
		signInCode(handler, adminPassword, "aaaaa-bbbbb")
	}
	locked := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "abcde-fghij"))
	if locked.Code != http.StatusUnauthorized || responseField(tester, locked, "error") != invalidLogin {
		tester.Fatalf("while locked: %d %s", locked.Code, locked.Body.String())
	}
	if left := recoveryCodesLeft(tester, handler, sessionCookie); left != float64(1) {
		tester.Fatalf("got %v codes left", left)
	}
}

// Turning TOTP off and on again must not bring the old printout back to life:
// the old set "stops working at once", whatever way the new one was made.
func TestReenablingTOTPReplacesRecoveryCodes(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	enableTOTP := func(code string) []string {
		tester.Helper()
		enabled := sendJSON(handler, http.MethodPost, "/api/account/totp",
			editorBody(map[string]any{"currentPassword": adminPassword, "secret": authTestSecret, "code": code}), sessionCookie)
		if enabled.Code != http.StatusOK {
			tester.Fatalf("enable: %d %s", enabled.Code, enabled.Body.String())
		}
		return recoveryCodesFrom(tester, enabled)
	}
	setClock(tester, time.Unix(59, 0))
	firstSet := enableTOTP("287082")
	setClock(tester, time.Unix(119, 0))
	if disabled := sendJSON(handler, http.MethodPost, "/api/account/totp/disable", confirmBody(adminPassword, "969429"), sessionCookie); disabled.Code != http.StatusNoContent {
		tester.Fatalf("disable: %d %s", disabled.Code, disabled.Body.String())
	}
	// Years later on the test clock: the session has gone idle, so sign in again (TOTP is off now).
	setClock(tester, time.Unix(1111111109, 0)) // RFC 6238: the code is 081804
	sessionCookie = sessionCookieFrom(tester, sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "")))
	secondSet := enableTOTP("081804")

	if left := recoveryCodesLeft(tester, handler, sessionCookie); left != float64(auth.RecoveryCodeCount) {
		tester.Fatalf("got %v codes left", left)
	}
	if signInCode(handler, adminPassword, firstSet[0]) != http.StatusUnauthorized {
		tester.Fatal("a code from the old set signed in")
	}
	if signInCode(handler, adminPassword, secondSet[0]) != http.StatusOK {
		tester.Fatal("a code from the new set must sign in")
	}
}
