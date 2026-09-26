package server

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

const adminPassword = "correct-password"

// RFC 6238 secret: at unix time 59 the valid code is 287082.
var authTestSecret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))

// failingStore is the real store with one method forced to fail, to test each error path.
type failingStore struct {
	*store.Store
	failingMethod string
}

var errStoreDown = errors.New("store down")

func (failing failingStore) RecordFailedLogin(ctx context.Context, accountID int64, maxFailures int, lockUntil time.Time) error {
	if failing.failingMethod == "RecordFailedLogin" {
		return errStoreDown
	}
	return failing.Store.RecordFailedLogin(ctx, accountID, maxFailures, lockUntil)
}

func (failing failingStore) ResetFailedLogins(ctx context.Context, accountID int64) error {
	if failing.failingMethod == "ResetFailedLogins" {
		return errStoreDown
	}
	return failing.Store.ResetFailedLogins(ctx, accountID)
}

func (failing failingStore) UseTOTPStep(ctx context.Context, accountID, timeStep int64) (bool, error) {
	if failing.failingMethod == "UseTOTPStep" {
		return false, errStoreDown
	}
	return failing.Store.UseTOTPStep(ctx, accountID, timeStep)
}

func (failing failingStore) CreateSession(ctx context.Context, tokenHash string, accountID int64, currentTime, expiresAt time.Time) error {
	if failing.failingMethod == "CreateSession" {
		return errStoreDown
	}
	return failing.Store.CreateSession(ctx, tokenHash, accountID, currentTime, expiresAt)
}

// openAdminStore returns a real SQLite store holding "admin" with adminPassword and the given TOTP secret.
func openAdminStore(tester *testing.T, totpSecret string) *store.Store {
	tester.Helper()
	testStore, err := store.Open(filepath.Join(tester.TempDir(), "docu-ui.db"))
	if err != nil {
		tester.Fatal(err)
	}
	tester.Cleanup(func() { testStore.Close() })
	if err := testStore.CreateFirstAccount(context.Background(), "admin", auth.HashPassword(adminPassword), totpSecret); err != nil {
		tester.Fatal(err)
	}
	return testStore
}

func newAuthServer(tester *testing.T, config Config) http.Handler {
	tester.Helper()
	handler, err := New(config, testUI)
	if err != nil {
		tester.Fatal(err)
	}
	return handler
}

// setClock fixes now() for one test.
func setClock(tester *testing.T, fixedTime time.Time) {
	now = func() time.Time { return fixedTime }
	tester.Cleanup(func() { now = time.Now })
}

func sendJSON(handler http.Handler, method, target, requestBody string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	handler.ServeHTTP(recorder, request)
	return recorder
}

func signInBody(username, password, totpCode string) string {
	encodedBody, _ := json.Marshal(map[string]string{"username": username, "password": password, "totpCode": totpCode})
	return string(encodedBody)
}

func sessionCookieFrom(tester *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	tester.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	tester.Fatalf("no session cookie in response %d %s", recorder.Code, recorder.Body.String())
	return nil
}

func responseField(tester *testing.T, recorder *httptest.ResponseRecorder, fieldName string) any {
	tester.Helper()
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		tester.Fatalf("response is not JSON: %q", recorder.Body.String())
	}
	return payload[fieldName]
}

func TestSignInWithPasswordStartsSession(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	signIn := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, ""))
	if signIn.Code != http.StatusOK {
		tester.Fatalf("got %d %s", signIn.Code, signIn.Body.String())
	}
	sessionCookie := sessionCookieFrom(tester, signIn)
	// HttpOnly keeps the token away from any injected script; Strict blocks cross-site requests.
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteStrictMode || sessionCookie.Path != "/" {
		tester.Fatalf("unsafe cookie: %+v", sessionCookie)
	}
	me := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie)
	if me.Code != http.StatusOK || responseField(tester, me, "username") != "admin" {
		tester.Fatalf("me: %d %s", me.Code, me.Body.String())
	}
}

// Behind a gateway sub-path the cookie must be scoped to it, not leak to other apps on the domain.
func TestSessionCookieFollowsBasePath(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, ""), BasePath: "/docu-ui", InsecureCookie: true})
	signIn := sendJSON(handler, http.MethodPost, "/docu-ui/api/auth/login", signInBody("admin", adminPassword, ""))
	sessionCookie := sessionCookieFrom(tester, signIn)
	if sessionCookie.Path != "/docu-ui/" || sessionCookie.Secure {
		tester.Fatalf("got path %q secure %v", sessionCookie.Path, sessionCookie.Secure)
	}
}

// Same answer for unknown user and wrong password, so sign-in cannot be used to list usernames.
func TestSignInDoesNotRevealUsernames(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	wrongPassword := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", "wrong-password", ""))
	unknownUser := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("nobody", "wrong-password", ""))
	if wrongPassword.Code != http.StatusUnauthorized || unknownUser.Code != http.StatusUnauthorized ||
		wrongPassword.Body.String() != unknownUser.Body.String() {
		tester.Fatalf("wrong password %d %s / unknown user %d %s",
			wrongPassword.Code, wrongPassword.Body.String(), unknownUser.Code, unknownUser.Body.String())
	}
	if len(wrongPassword.Result().Cookies()) != 0 {
		tester.Fatal("failed sign-in set a cookie")
	}
}

// Five wrong passwords lock the account even for the right password, until the lock expires.
func TestFailedSignInsLockAccount(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	startTime := time.Unix(1_000_000, 0)
	setClock(tester, startTime)
	for attempt := 1; attempt <= maxFailedLogins; attempt++ {
		sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", "guess", ""))
	}
	locked := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, ""))
	if locked.Code != http.StatusTooManyRequests || !strings.Contains(locked.Body.String(), "15 minute") {
		tester.Fatalf("while locked: %d %s", locked.Code, locked.Body.String())
	}
	setClock(tester, startTime.Add(loginLockoutTime+time.Second))
	if unlocked := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "")); unlocked.Code != http.StatusOK {
		tester.Fatalf("after lock expired: %d %s", unlocked.Code, unlocked.Body.String())
	}
}

func TestSignInWithTOTP(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, authTestSecret)})
	setClock(tester, time.Unix(59, 0))

	// Asking for the code is not a failure: many "right password, no code yet" answers must not lock the account.
	for attempt := 1; attempt <= maxFailedLogins+1; attempt++ {
		passwordOnly := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, ""))
		if passwordOnly.Code != http.StatusUnauthorized || responseField(tester, passwordOnly, "totpRequired") != true {
			tester.Fatalf("password only: %d %s", passwordOnly.Code, passwordOnly.Body.String())
		}
	}
	wrongCode := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "000000"))
	if wrongCode.Code != http.StatusUnauthorized || responseField(tester, wrongCode, "totpRequired") != true || len(wrongCode.Result().Cookies()) != 0 {
		tester.Fatalf("wrong code: %d %s", wrongCode.Code, wrongCode.Body.String())
	}
	rightCode := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "287082"))
	if rightCode.Code != http.StatusOK {
		tester.Fatalf("right code: %d %s", rightCode.Code, rightCode.Body.String())
	}
	sessionCookieFrom(tester, rightCode)
	// Someone who saw the code (shoulder surfing, proxy log) cannot reuse it within its 30s window.
	replayed := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "287082"))
	if replayed.Code != http.StatusUnauthorized || !strings.Contains(replayed.Body.String(), "already used") {
		tester.Fatalf("replayed code: %d %s", replayed.Code, replayed.Body.String())
	}
}

// A wrong password must be rejected before TOTP is even asked, so the code prompt confirms nothing.
func TestTOTPAccountWithWrongPassword(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, authTestSecret)})
	wrongPassword := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", "wrong-password", ""))
	if wrongPassword.Code != http.StatusUnauthorized || responseField(tester, wrongPassword, "totpRequired") != nil {
		tester.Fatalf("got %d %s", wrongPassword.Code, wrongPassword.Body.String())
	}
}

func TestMeRequiresLiveSession(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	startTime := time.Unix(1_000_000, 0)
	setClock(tester, startTime)
	sessionCookie := sessionCookieFrom(tester, sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "")))

	if noCookie := sendJSON(handler, http.MethodGet, "/api/auth/me", ""); noCookie.Code != http.StatusUnauthorized {
		tester.Errorf("no cookie: %d", noCookie.Code)
	}
	forgedCookie := &http.Cookie{Name: sessionCookieName, Value: "made-up-token"}
	if forged := sendJSON(handler, http.MethodGet, "/api/auth/me", "", forgedCookie); forged.Code != http.StatusUnauthorized {
		tester.Errorf("forged cookie: %d", forged.Code)
	}
	setClock(tester, startTime.Add(sessionLifetime))
	if expired := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie); expired.Code != http.StatusUnauthorized {
		tester.Errorf("expired session: %d", expired.Code)
	}
}

// Sign-out must kill the session on the server, not only in the browser: a copied cookie stops working too.
func TestSignOutEndsSession(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	sessionCookie := sessionCookieFrom(tester, sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "")))

	signOut := sendJSON(handler, http.MethodPost, "/api/auth/logout", "", sessionCookie)
	if signOut.Code != http.StatusNoContent || sessionCookieFrom(tester, signOut).MaxAge >= 0 {
		tester.Fatalf("sign-out: %d, cookie %+v", signOut.Code, sessionCookieFrom(tester, signOut))
	}
	if me := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie); me.Code != http.StatusUnauthorized {
		tester.Fatalf("old cookie still works after sign-out: %d", me.Code)
	}
	if withoutSession := sendJSON(handler, http.MethodPost, "/api/auth/logout", ""); withoutSession.Code != http.StatusNoContent {
		tester.Fatalf("sign-out without session: %d", withoutSession.Code)
	}
}

func TestSignInRequiresJSON(tester *testing.T) {
	handler := newAuthServer(tester, Config{Store: openAdminStore(tester, "")})
	recorder := httptest.NewRecorder()
	formRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("username=admin"))
	formRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(recorder, formRequest)
	if recorder.Code != http.StatusUnsupportedMediaType {
		tester.Errorf("form post: %d", recorder.Code)
	}
	if brokenJSON := sendJSON(handler, http.MethodPost, "/api/auth/login", "{not json"); brokenJSON.Code != http.StatusBadRequest {
		tester.Errorf("broken JSON: %d", brokenJSON.Code)
	}
}

func TestAuthReportsStoreFailures(tester *testing.T) {
	setClock(tester, time.Unix(59, 0))
	failingCases := map[string]struct {
		totpSecret string
		requestFn  func(handler http.Handler) *httptest.ResponseRecorder
	}{
		"RecordFailedLogin": {"", func(handler http.Handler) *httptest.ResponseRecorder {
			return sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", "wrong-password", ""))
		}},
		"ResetFailedLogins": {"", func(handler http.Handler) *httptest.ResponseRecorder {
			return sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, ""))
		}},
		"CreateSession": {"", func(handler http.Handler) *httptest.ResponseRecorder {
			return sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, ""))
		}},
		"UseTOTPStep": {authTestSecret, func(handler http.Handler) *httptest.ResponseRecorder {
			return sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "287082"))
		}},
	}
	for failingMethod, failingCase := range failingCases {
		failing := failingStore{Store: openAdminStore(tester, failingCase.totpSecret), failingMethod: failingMethod}
		recorder := failingCase.requestFn(newAuthServer(tester, Config{Store: failing}))
		if recorder.Code != http.StatusInternalServerError || len(recorder.Result().Cookies()) != 0 {
			tester.Errorf("%s failing: got %d %s", failingMethod, recorder.Code, recorder.Body.String())
		}
	}

	// A closed database fails every read.
	closedStore := openAdminStore(tester, "")
	handler := newAuthServer(tester, Config{Store: closedStore})
	sessionCookie := sessionCookieFrom(tester, sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "")))
	closedStore.Close()
	if signIn := sendJSON(handler, http.MethodPost, "/api/auth/login", signInBody("admin", adminPassword, "")); signIn.Code != http.StatusInternalServerError {
		tester.Errorf("sign-in on closed store: %d", signIn.Code)
	}
	if me := sendJSON(handler, http.MethodGet, "/api/auth/me", "", sessionCookie); me.Code != http.StatusInternalServerError {
		tester.Errorf("me on closed store: %d", me.Code)
	}
	if signOut := sendJSON(handler, http.MethodPost, "/api/auth/logout", "", sessionCookie); signOut.Code != http.StatusInternalServerError {
		tester.Errorf("sign-out on closed store: %d", signOut.Code)
	}
}

func (failing failingStore) FindAccount(ctx context.Context, username string) (store.Account, error) {
	if failing.failingMethod == "FindAccount" {
		return store.Account{}, errStoreDown
	}
	return failing.Store.FindAccount(ctx, username)
}

func (failing failingStore) SetPassword(ctx context.Context, accountID int64, passwordHash string) error {
	if failing.failingMethod == "SetPassword" {
		return errStoreDown
	}
	return failing.Store.SetPassword(ctx, accountID, passwordHash)
}

func (failing failingStore) DeleteOtherSessions(ctx context.Context, accountID int64, keepTokenHash string) error {
	if failing.failingMethod == "DeleteOtherSessions" {
		return errStoreDown
	}
	return failing.Store.DeleteOtherSessions(ctx, accountID, keepTokenHash)
}

func (failing failingStore) SetTOTP(ctx context.Context, accountID int64, secret string, lastStep int64) error {
	if failing.failingMethod == "SetTOTP" {
		return errStoreDown
	}
	return failing.Store.SetTOTP(ctx, accountID, secret, lastStep)
}

func (failing failingStore) SignInOff(ctx context.Context) (bool, error) {
	if failing.failingMethod == "SignInOff" {
		return false, errStoreDown
	}
	return failing.Store.SignInOff(ctx)
}

func (failing failingStore) TurnOffSignIn(ctx context.Context) error {
	if failing.failingMethod == "TurnOffSignIn" {
		return errStoreDown
	}
	return failing.Store.TurnOffSignIn(ctx)
}

func (failing failingStore) CreateFirstAccount(ctx context.Context, username, passwordHash, totpSecret string) error {
	switch failing.failingMethod {
	case "CreateFirstAccount":
		return errStoreDown
	case "CreateFirstAccountRace":
		// Another request created the account between the check and the insert.
		return store.ErrSetupDone
	}
	return failing.Store.CreateFirstAccount(ctx, username, passwordHash, totpSecret)
}

func (failing failingStore) FindSessionUsername(ctx context.Context, tokenHash string, currentTime time.Time) (string, error) {
	if failing.failingMethod == "FindSessionUsername" {
		return "", errStoreDown
	}
	return failing.Store.FindSessionUsername(ctx, tokenHash, currentTime)
}
