package server

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

// Wrong confirmations answer 403, not 401: the session is fine, and the UI sends 401 back to sign-in.
const (
	wrongCurrentPassword = "current password is wrong"
	wrongTOTPCode        = "TOTP code is wrong"
	cannotAccounts       = "cannot read accounts"
	cannotSaveAccount    = "cannot save account"
)

func (handlers authHandlers) registerAccount(routes *http.ServeMux) {
	routes.Handle("GET /api/account", handlers.requireSession(handlers.account))
	routes.Handle("PUT /api/account/password", handlers.requireSession(handlers.changePassword))
	routes.Handle("GET /api/account/totp-secret", handlers.requireSession(handlers.newTOTPSecret))
	routes.Handle("POST /api/account/totp", handlers.requireSession(handlers.enableTOTP))
	routes.Handle("POST /api/account/totp/disable", handlers.requireSession(handlers.disableTOTP))
	routes.Handle("POST /api/account/sign-in", handlers.requireSession(handlers.turnOnSignIn))
	routes.Handle("POST /api/account/sign-in/disable", handlers.requireSession(handlers.turnOffSignIn))
}

func (handlers authHandlers) account(writer http.ResponseWriter, request *http.Request, username string) {
	if isSignInOff(request) {
		writeJSON(writer, http.StatusOK, map[string]any{"signIn": false})
		return
	}
	account, err := handlers.store.FindAccount(request.Context(), username)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotAccounts)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"signIn": true, "username": account.Username, "totpEnabled": account.TOTPSecret != ""})
}

// turnOnSignIn creates the account while sign-in is off. From then on every request needs a
// session, this one's browser included, so the UI goes to sign-in next.
func (handlers authHandlers) turnOnSignIn(writer http.ResponseWriter, request *http.Request, _ string) {
	var accountInput struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(writer, request, &accountInput) {
		return
	}
	if !isSignInOff(request) {
		writeError(writer, http.StatusConflict, "sign-in is already on")
		return
	}
	if message := validateSetup(setupRequest{Username: accountInput.Username, Password: accountInput.Password}); message != "" {
		writeError(writer, http.StatusBadRequest, message)
		return
	}
	err := handlers.store.CreateFirstAccount(request.Context(), accountInput.Username, auth.HashPassword(accountInput.Password), "")
	if errors.Is(err, store.ErrSetupDone) {
		writeError(writer, http.StatusConflict, "sign-in is already on")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	slog.Info("sign-in turned on", "user", accountInput.Username)
	writeJSON(writer, http.StatusCreated, map[string]string{"username": accountInput.Username})
}

// turnOffSignIn needs the password, and a code when TOTP is on: the same proof as signing in.
// It deletes the account and every session.
func (handlers authHandlers) turnOffSignIn(writer http.ResponseWriter, request *http.Request, username string) {
	var confirmInput struct {
		CurrentPassword string `json:"currentPassword"`
		Code            string `json:"code"`
	}
	if !decodeJSON(writer, request, &confirmInput) {
		return
	}
	account, ok := handlers.confirmPassword(writer, request, username, confirmInput.CurrentPassword)
	if !ok {
		return
	}
	if account.TOTPSecret != "" && !handlers.confirmTOTPCode(writer, request, account, confirmInput.Code) {
		return
	}
	if err := handlers.store.TurnOffSignIn(request.Context()); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	slog.Warn(SignInOffWarning, "turned_off_by", username)
	handlers.clearSessionCookie(writer)
	writer.WriteHeader(http.StatusNoContent)
}

// changePassword needs the current password, so a session left open on another screen cannot take
// the account over. Every other session ends: whoever knew the old password is signed out.
func (handlers authHandlers) changePassword(writer http.ResponseWriter, request *http.Request, username string) {
	var passwordInput struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(writer, request, &passwordInput) {
		return
	}
	if !isValidPasswordLength(passwordInput.NewPassword) {
		writeError(writer, http.StatusBadRequest, "new "+passwordLengthProblem)
		return
	}
	account, ok := handlers.confirmPassword(writer, request, username, passwordInput.CurrentPassword)
	if !ok {
		return
	}
	ctx := request.Context()
	if err := handlers.store.SetPassword(ctx, account.ID, auth.HashPassword(passwordInput.NewPassword)); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	// requireSession already checked this cookie.
	sessionCookie, _ := request.Cookie(sessionCookieName)
	if err := handlers.store.DeleteOtherSessions(ctx, account.ID, hashSessionToken(sessionCookie.Value)); err != nil {
		writeError(writer, http.StatusInternalServerError, "password changed, but cannot sign out other sessions")
		return
	}
	slog.Info("password changed", "user", username)
	writer.WriteHeader(http.StatusNoContent)
}

// newTOTPSecret hands out a fresh secret for the QR code. Nothing is stored until enableTOTP.
func (handlers authHandlers) newTOTPSecret(writer http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(writer, http.StatusOK, map[string]string{"secret": auth.NewTOTPSecret()})
}

// enableTOTP needs the current password and a code from the app, which proves the app was set up.
func (handlers authHandlers) enableTOTP(writer http.ResponseWriter, request *http.Request, username string) {
	var totpInput struct {
		CurrentPassword string `json:"currentPassword"`
		Secret          string `json:"secret"`
		Code            string `json:"code"`
	}
	if !decodeJSON(writer, request, &totpInput) {
		return
	}
	if !auth.ValidTOTPSecret(totpInput.Secret) {
		writeError(writer, http.StatusBadRequest, "TOTP secret is invalid")
		return
	}
	account, ok := handlers.confirmPassword(writer, request, username, totpInput.CurrentPassword)
	if !ok {
		return
	}
	if account.TOTPSecret != "" {
		writeError(writer, http.StatusConflict, "TOTP is already on")
		return
	}
	codeStep, codeMatched := auth.MatchTOTPStep(totpInput.Secret, totpInput.Code, now())
	if !codeMatched {
		writeError(writer, http.StatusBadRequest, "TOTP code does not match: check the time on your phone and try the next code")
		return
	}
	if err := handlers.store.SetTOTP(request.Context(), account.ID, totpInput.Secret, codeStep); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	slog.Info("totp enabled", "user", username)
	writer.WriteHeader(http.StatusNoContent)
}

// disableTOTP needs the current password and a current code: both factors, like signing in.
func (handlers authHandlers) disableTOTP(writer http.ResponseWriter, request *http.Request, username string) {
	var totpInput struct {
		CurrentPassword string `json:"currentPassword"`
		Code            string `json:"code"`
	}
	if !decodeJSON(writer, request, &totpInput) {
		return
	}
	account, ok := handlers.confirmPassword(writer, request, username, totpInput.CurrentPassword)
	if !ok {
		return
	}
	if account.TOTPSecret == "" {
		writeError(writer, http.StatusConflict, "TOTP is already off")
		return
	}
	if !handlers.confirmTOTPCode(writer, request, account, totpInput.Code) {
		return
	}
	if err := handlers.store.SetTOTP(request.Context(), account.ID, "", 0); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	slog.Info("totp disabled", "user", username)
	writer.WriteHeader(http.StatusNoContent)
}

// confirmPassword checks the signed-in user's current password. Wrong guesses count towards the
// same lockout as sign-in, so a hijacked session cannot brute-force the password here.
func (handlers authHandlers) confirmPassword(writer http.ResponseWriter, request *http.Request, username, currentPassword string) (store.Account, bool) {
	if isSignInOff(request) {
		writeError(writer, http.StatusConflict, "sign-in is off: there is no account")
		return store.Account{}, false
	}
	account, err := handlers.store.FindAccount(request.Context(), username)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotAccounts)
		return store.Account{}, false
	}
	currentTime := now()
	if account.LockedUntil.After(currentTime) {
		minutesLeft := int(math.Ceil(account.LockedUntil.Sub(currentTime).Minutes()))
		writeError(writer, http.StatusTooManyRequests, fmt.Sprintf("too many wrong attempts: try again in %d minute(s)", minutesLeft))
		return store.Account{}, false
	}
	if passwordMatched, _ := auth.VerifyPassword(currentPassword, account.PasswordHash); !passwordMatched {
		handlers.failConfirmation(writer, request, account, wrongCurrentPassword)
		return store.Account{}, false
	}
	return account, true
}

// confirmTOTPCode checks a code from the account's app; a code already used does not count.
func (handlers authHandlers) confirmTOTPCode(writer http.ResponseWriter, request *http.Request, account store.Account, code string) bool {
	codeStep, codeMatched := auth.MatchTOTPStep(account.TOTPSecret, code, now())
	if !codeMatched {
		handlers.failConfirmation(writer, request, account, wrongTOTPCode)
		return false
	}
	stepAccepted, err := handlers.store.UseTOTPStep(request.Context(), account.ID, codeStep)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return false
	}
	if !stepAccepted {
		handlers.failConfirmation(writer, request, account, "TOTP code was already used: wait for the next code")
		return false
	}
	return true
}

// failConfirmation counts a wrong password or code towards the lockout and answers 403.
func (handlers authHandlers) failConfirmation(writer http.ResponseWriter, request *http.Request, account store.Account, message string) {
	if err := handlers.store.RecordFailedLogin(request.Context(), account.ID, maxFailedLogins, now().Add(loginLockoutTime)); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	writeError(writer, http.StatusForbidden, message)
}
