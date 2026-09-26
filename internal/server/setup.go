package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

// Store is the persistence the HTTP layer needs (implemented by internal/store).
type Store interface {
	NeedsSetup(ctx context.Context) (bool, error)
	CreateFirstAccount(ctx context.Context, username, passwordHash, totpSecret string) error
	FindAccount(ctx context.Context, username string) (store.Account, error)
	RecordFailedLogin(ctx context.Context, accountID int64, maxFailures int, lockUntil time.Time) error
	ResetFailedLogins(ctx context.Context, accountID int64) error
	UseTOTPStep(ctx context.Context, accountID, timeStep int64) (bool, error)
	CreateSession(ctx context.Context, tokenHash string, accountID int64, now, expiresAt time.Time) error
	FindSessionUsername(ctx context.Context, tokenHash string, now time.Time) (string, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteOtherSessions(ctx context.Context, accountID int64, keepTokenHash string) error
	SetPassword(ctx context.Context, accountID int64, passwordHash string) error
	SetTOTP(ctx context.Context, accountID int64, secret string, lastStep int64) error
	EnvFolder(ctx context.Context) (string, error)
	SetEnvFolder(ctx context.Context, folder string) error
	DocoCD(ctx context.Context) (store.DocoCD, error)
	SetDocoCD(ctx context.Context, settings store.DocoCD) error
	Webhook(ctx context.Context) (store.Webhook, error)
	SetWebhook(ctx context.Context, webhook store.Webhook) error
	ApplyTarget(ctx context.Context, fileName string) (store.ApplyTarget, error)
	SetApplyTarget(ctx context.Context, fileName string, target store.ApplyTarget) error
}

const (
	minPasswordLength = 12
	maxPasswordLength = 256
	maxRequestBytes   = 64 << 10
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

// now is replaced in tests to check TOTP codes at a fixed time.
var now = time.Now

type setupRequest struct {
	SetupToken string `json:"setupToken"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	// TOTPSecret is empty when the admin skips two-factor; otherwise TOTPCode must prove the app was set up.
	TOTPSecret string `json:"totpSecret"`
	TOTPCode   string `json:"totpCode"`
}

type setupHandlers struct {
	accounts   Store
	setupToken string
}

func (handlers setupHandlers) register(routes *http.ServeMux) {
	routes.HandleFunc("GET /api/setup", handlers.status)
	routes.HandleFunc("GET /api/setup/totp-secret", handlers.totpSecret)
	routes.HandleFunc("POST /api/setup", handlers.createFirstAccount)
}

// status tells the UI whether to show the setup page instead of sign-in.
func (handlers setupHandlers) status(writer http.ResponseWriter, request *http.Request) {
	needsSetup, err := handlers.accounts.NeedsSetup(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot read accounts")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]bool{"required": needsSetup})
}

// totpSecret hands out a fresh secret for the QR code. Nothing is stored until setup is submitted.
func (handlers setupHandlers) totpSecret(writer http.ResponseWriter, request *http.Request) {
	if !handlers.setupOpen(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"secret": auth.NewTOTPSecret()})
}

func (handlers setupHandlers) createFirstAccount(writer http.ResponseWriter, request *http.Request) {
	var setupInput setupRequest
	if !decodeJSON(writer, request, &setupInput) {
		return
	}
	if !handlers.setupOpen(writer, request) {
		return
	}
	if handlers.setupToken == "" ||
		subtle.ConstantTimeCompare([]byte(setupInput.SetupToken), []byte(handlers.setupToken)) != 1 {
		writeError(writer, http.StatusForbidden, "setup token is wrong: copy it from the server log")
		return
	}
	if message := validateSetup(setupInput); message != "" {
		writeError(writer, http.StatusBadRequest, message)
		return
	}
	err := handlers.accounts.CreateFirstAccount(request.Context(),
		setupInput.Username, auth.HashPassword(setupInput.Password), setupInput.TOTPSecret)
	if errors.Is(err, store.ErrSetupDone) {
		writeError(writer, http.StatusConflict, "setup already completed")
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save account")
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]string{"username": setupInput.Username})
}

// setupOpen writes the error response and returns false once an account exists.
func (handlers setupHandlers) setupOpen(writer http.ResponseWriter, request *http.Request) bool {
	needsSetup, err := handlers.accounts.NeedsSetup(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot read accounts")
		return false
	}
	if !needsSetup {
		writeError(writer, http.StatusConflict, "setup already completed")
		return false
	}
	return true
}

// validateSetup returns a user-facing message for the first invalid field, or "".
func validateSetup(setupInput setupRequest) string {
	if !usernamePattern.MatchString(setupInput.Username) {
		return "username must be 3-64 characters: letters, digits, dot, dash, underscore"
	}
	if !isValidPasswordLength(setupInput.Password) {
		return passwordLengthProblem
	}
	if setupInput.TOTPSecret == "" {
		return ""
	}
	if !auth.ValidTOTPSecret(setupInput.TOTPSecret) {
		return "TOTP secret is invalid"
	}
	if !auth.VerifyTOTP(setupInput.TOTPSecret, setupInput.TOTPCode, now()) {
		return "TOTP code does not match: check the time on your phone and try the next code"
	}
	return ""
}

const passwordLengthProblem = "password must be 12-256 characters"

func isValidPasswordLength(password string) bool {
	passwordLength := utf8.RuneCountInString(password)
	return passwordLength >= minPasswordLength && passwordLength <= maxPasswordLength
}

// decodeJSON reads a JSON request body into target, or answers 415/400 and returns false.
func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	// Browsers cannot send application/json cross-site without a CORS preflight, which we never allow.
	if mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type")); mediaType != "application/json" {
		writeError(writer, http.StatusUnsupportedMediaType, "content type must be application/json")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxRequestBytes)).Decode(target); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, statusCode int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, statusCode int, message string) {
	writeJSON(writer, statusCode, map[string]string{"error": message})
}
