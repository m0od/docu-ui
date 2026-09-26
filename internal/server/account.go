package server

import (
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
}

func (handlers authHandlers) account(writer http.ResponseWriter, request *http.Request, username string) {
	account, err := handlers.store.FindAccount(request.Context(), username)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotAccounts)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"username": account.Username, "totpEnabled": account.TOTPSecret != ""})
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
	ctx := request.Context()
	codeStep, codeMatched := auth.MatchTOTPStep(account.TOTPSecret, totpInput.Code, now())
	if !codeMatched {
		handlers.failConfirmation(writer, request, account, wrongTOTPCode)
		return
	}
	stepAccepted, err := handlers.store.UseTOTPStep(ctx, account.ID, codeStep)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	if !stepAccepted {
		handlers.failConfirmation(writer, request, account, "TOTP code was already used: wait for the next code")
		return
	}
	if err := handlers.store.SetTOTP(ctx, account.ID, "", 0); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	slog.Info("totp disabled", "user", username)
	writer.WriteHeader(http.StatusNoContent)
}

// confirmPassword checks the signed-in user's current password. Wrong guesses count towards the
// same lockout as sign-in, so a hijacked session cannot brute-force the password here.
func (handlers authHandlers) confirmPassword(writer http.ResponseWriter, request *http.Request, username, currentPassword string) (store.Account, bool) {
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

// failConfirmation counts a wrong password or code towards the lockout and answers 403.
func (handlers authHandlers) failConfirmation(writer http.ResponseWriter, request *http.Request, account store.Account, message string) {
	if err := handlers.store.RecordFailedLogin(request.Context(), account.ID, maxFailedLogins, now().Add(loginLockoutTime)); err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSaveAccount)
		return
	}
	writeError(writer, http.StatusForbidden, message)
}
