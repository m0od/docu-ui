package server

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

const testSetupToken = "setup-token-from-log"

// fakeAccounts mimics the store: setup is open until the first account is created.
type fakeAccounts struct {
	Store             // methods setup never calls; a call would panic and fail the test
	createdUsername   string
	createdPassword   string
	createdTOTPSecret string
	readError         error
	writeError        error
}

func (accounts *fakeAccounts) NeedsSetup(context.Context) (bool, error) {
	return accounts.createdUsername == "", accounts.readError
}

func (accounts *fakeAccounts) CreateFirstAccount(_ context.Context, username, passwordHash, totpSecret string) error {
	if accounts.writeError != nil {
		return accounts.writeError
	}
	accounts.createdUsername, accounts.createdPassword, accounts.createdTOTPSecret = username, passwordHash, totpSecret
	return nil
}

func newSetupServer(tester *testing.T, accounts *fakeAccounts, setupToken string) http.Handler {
	tester.Helper()
	handler, err := New(Config{Store: accounts, SetupToken: setupToken}, testUI)
	if err != nil {
		tester.Fatal(err)
	}
	return handler
}

func postSetup(handler http.Handler, contentType, requestBody string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", contentType)
	handler.ServeHTTP(recorder, request)
	return recorder
}

func setupBody(fields map[string]string) string {
	requestFields := map[string]string{"setupToken": testSetupToken, "username": "admin", "password": "a-long-password"}
	for fieldName, fieldValue := range fields {
		requestFields[fieldName] = fieldValue
	}
	encodedBody, _ := json.Marshal(requestFields)
	return string(encodedBody)
}

func errorMessage(tester *testing.T, recorder *httptest.ResponseRecorder) string {
	tester.Helper()
	var errorPayload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &errorPayload); err != nil {
		tester.Fatalf("response is not JSON: %q", recorder.Body.String())
	}
	return errorPayload["error"]
}

func TestSetupStatusFollowsStore(tester *testing.T) {
	accounts := &fakeAccounts{}
	handler := newSetupServer(tester, accounts, testSetupToken)
	if body := readBody(tester, get(tester, handler, "/api/setup")); strings.TrimSpace(body) != `{"required":true}` {
		tester.Fatalf("fresh install: %s", body)
	}
	accounts.createdUsername = "admin"
	if body := readBody(tester, get(tester, handler, "/api/setup")); strings.TrimSpace(body) != `{"required":false}` {
		tester.Fatalf("after setup: %s", body)
	}
}

// The account is stored with a hash, never the plain password.
func TestSetupCreatesAccountWithoutTOTP(tester *testing.T) {
	accounts := &fakeAccounts{}
	recorder := postSetup(newSetupServer(tester, accounts, testSetupToken), "application/json", setupBody(nil))
	if recorder.Code != http.StatusCreated {
		tester.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	if accounts.createdUsername != "admin" || accounts.createdTOTPSecret != "" {
		tester.Fatalf("stored %q / totp %q", accounts.createdUsername, accounts.createdTOTPSecret)
	}
	if matched, _ := auth.VerifyPassword("a-long-password", accounts.createdPassword); !matched {
		tester.Fatalf("stored password %q is not a hash of the submitted one", accounts.createdPassword)
	}
}

func TestSetupWithTOTP(tester *testing.T) {
	// RFC 6238 secret: at unix time 59 the valid code is 287082.
	rfcSecret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	now = func() time.Time { return time.Unix(59, 0) }
	tester.Cleanup(func() { now = time.Now })

	accounts := &fakeAccounts{}
	handler := newSetupServer(tester, accounts, testSetupToken)
	// A wrong code means the phone app is not set up; saving the secret would lock the admin out.
	wrongCode := postSetup(handler, "application/json", setupBody(map[string]string{"totpSecret": rfcSecret, "totpCode": "000000"}))
	if wrongCode.Code != http.StatusBadRequest || accounts.createdUsername != "" {
		tester.Fatalf("wrong code: %d, created %q", wrongCode.Code, accounts.createdUsername)
	}
	rightCode := postSetup(handler, "application/json", setupBody(map[string]string{"totpSecret": rfcSecret, "totpCode": "287082"}))
	if rightCode.Code != http.StatusCreated || accounts.createdTOTPSecret != rfcSecret {
		tester.Fatalf("right code: %d %s", rightCode.Code, rightCode.Body.String())
	}
}

// Without the token from the server log, a stranger who finds the page first cannot become admin.
func TestSetupRequiresToken(tester *testing.T) {
	for caseName, serverToken := range map[string]string{"wrong token": testSetupToken, "no token configured": ""} {
		accounts := &fakeAccounts{}
		recorder := postSetup(newSetupServer(tester, accounts, serverToken), "application/json",
			setupBody(map[string]string{"setupToken": "guess"}))
		if recorder.Code != http.StatusForbidden || accounts.createdUsername != "" {
			tester.Errorf("%s: got %d, created %q", caseName, recorder.Code, accounts.createdUsername)
		}
	}
}

// Once an admin exists, setup must stay closed even with the right token.
func TestSetupClosedAfterFirstAccount(tester *testing.T) {
	accounts := &fakeAccounts{createdUsername: "admin"}
	handler := newSetupServer(tester, accounts, testSetupToken)
	if recorder := postSetup(handler, "application/json", setupBody(map[string]string{"username": "second"})); recorder.Code != http.StatusConflict {
		tester.Fatalf("second setup: %d", recorder.Code)
	}
	if response := get(tester, handler, "/api/setup/totp-secret"); response.StatusCode != http.StatusConflict {
		tester.Fatalf("totp secret after setup: %d", response.StatusCode)
	}
	if accounts.createdUsername != "admin" {
		tester.Fatalf("admin was replaced by %q", accounts.createdUsername)
	}
}

// Two requests can both pass the NeedsSetup check; the store's atomic insert is the final guard.
func TestSetupLosesRaceToAnotherRequest(tester *testing.T) {
	accounts := &fakeAccounts{writeError: store.ErrSetupDone}
	if recorder := postSetup(newSetupServer(tester, accounts, testSetupToken), "application/json", setupBody(nil)); recorder.Code != http.StatusConflict {
		tester.Fatalf("got %d", recorder.Code)
	}
}

func TestSetupRejectsInvalidInput(tester *testing.T) {
	invalidFields := map[string]map[string]string{
		"short username":   {"username": "ab"},
		"username symbols": {"username": "admin; drop"},
		"short password":   {"password": "short"},
		"long password":    {"password": strings.Repeat("p", maxPasswordLength+1)},
		"bad totp secret":  {"totpSecret": "not base32!", "totpCode": "123456"},
	}
	for caseName, fields := range invalidFields {
		accounts := &fakeAccounts{}
		recorder := postSetup(newSetupServer(tester, accounts, testSetupToken), "application/json", setupBody(fields))
		if recorder.Code != http.StatusBadRequest || errorMessage(tester, recorder) == "" || accounts.createdUsername != "" {
			tester.Errorf("%s: got %d %s", caseName, recorder.Code, recorder.Body.String())
		}
	}
}

// A form posted from another site arrives as form data; refusing it blocks CSRF (cross-site request forgery).
func TestSetupRequiresJSON(tester *testing.T) {
	handler := newSetupServer(tester, &fakeAccounts{}, testSetupToken)
	if recorder := postSetup(handler, "application/x-www-form-urlencoded", "username=admin"); recorder.Code != http.StatusUnsupportedMediaType {
		tester.Fatalf("form post: %d", recorder.Code)
	}
	if recorder := postSetup(handler, "application/json", "{not json"); recorder.Code != http.StatusBadRequest {
		tester.Fatalf("broken JSON: %d", recorder.Code)
	}
}

func TestSetupTOTPSecretIsFresh(tester *testing.T) {
	handler := newSetupServer(tester, &fakeAccounts{}, testSetupToken)
	var firstSecret, secondSecret map[string]string
	json.Unmarshal([]byte(readBody(tester, get(tester, handler, "/api/setup/totp-secret"))), &firstSecret)
	json.Unmarshal([]byte(readBody(tester, get(tester, handler, "/api/setup/totp-secret"))), &secondSecret)
	if !auth.ValidTOTPSecret(firstSecret["secret"]) || firstSecret["secret"] == secondSecret["secret"] {
		tester.Fatalf("got %v and %v", firstSecret, secondSecret)
	}
}

func TestSetupReportsStoreFailures(tester *testing.T) {
	brokenRead := newSetupServer(tester, &fakeAccounts{readError: errors.New("disk gone")}, testSetupToken)
	if response := get(tester, brokenRead, "/api/setup"); response.StatusCode != http.StatusInternalServerError {
		tester.Errorf("status: %d", response.StatusCode)
	}
	if response := get(tester, brokenRead, "/api/setup/totp-secret"); response.StatusCode != http.StatusInternalServerError {
		tester.Errorf("totp secret: %d", response.StatusCode)
	}
	brokenWrite := newSetupServer(tester, &fakeAccounts{writeError: errors.New("disk full")}, testSetupToken)
	if recorder := postSetup(brokenWrite, "application/json", setupBody(nil)); recorder.Code != http.StatusInternalServerError {
		tester.Errorf("create: %d", recorder.Code)
	}
}

// A typo in an API path must not return the HTML page, or the UI would try to parse HTML as JSON.
func TestUnknownAPIPathIsJSON404(tester *testing.T) {
	response := get(tester, mustNew(tester, ""), "/api/nope")
	if response.StatusCode != http.StatusNotFound || response.Header.Get("Content-Type") != "application/json" {
		tester.Fatalf("got %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
}
