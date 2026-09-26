package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

const (
	sessionCookieName = "docu_session"
	sessionLifetime   = 12 * time.Hour
	// Five wrong passwords or TOTP codes lock the account for 15 minutes: slow enough to stop
	// guessing, short enough that a locked-out admin just waits.
	maxFailedLogins  = 5
	loginLockoutTime = 15 * time.Minute
	invalidLogin     = "invalid username or password"
)

// dummyPasswordHash is checked when the username does not exist, so a wrong username takes as
// long as a wrong password and response time does not reveal which usernames exist.
var dummyPasswordHash = sync.OnceValue(func() string { return auth.HashPassword("docu-ui-dummy-password") })

type signInRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// TOTPCode is sent on the second step, after the server answered totpRequired.
	TOTPCode string `json:"totpCode"`
}

type authHandlers struct {
	store        Store
	cookiePath   string
	secureCookie bool
}

func (handlers authHandlers) register(routes *http.ServeMux) {
	routes.HandleFunc("POST /api/auth/login", handlers.signIn)
	routes.HandleFunc("POST /api/auth/logout", handlers.signOut)
	routes.Handle("GET /api/auth/me", handlers.requireSession(func(writer http.ResponseWriter, _ *http.Request, username string) {
		writeJSON(writer, http.StatusOK, map[string]string{"username": username})
	}))
	handlers.registerAccount(routes)
}

func (handlers authHandlers) signIn(writer http.ResponseWriter, request *http.Request) {
	var signInInput signInRequest
	if !decodeJSON(writer, request, &signInInput) {
		return
	}
	ctx := request.Context()
	currentTime := now()

	account, err := handlers.store.FindAccount(ctx, signInInput.Username)
	if errors.Is(err, store.ErrAccountNotFound) {
		_, _ = auth.VerifyPassword(signInInput.Password, dummyPasswordHash())
		writeError(writer, http.StatusUnauthorized, invalidLogin)
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot read accounts")
		return
	}
	if account.LockedUntil.After(currentTime) {
		minutesLeft := int(math.Ceil(account.LockedUntil.Sub(currentTime).Minutes()))
		writeError(writer, http.StatusTooManyRequests, fmt.Sprintf("too many failed sign-ins: try again in %d minute(s)", minutesLeft))
		return
	}
	if passwordMatched, _ := auth.VerifyPassword(signInInput.Password, account.PasswordHash); !passwordMatched {
		handlers.failSignIn(writer, request, account, invalidLogin, false)
		return
	}
	if account.TOTPSecret != "" {
		if signInInput.TOTPCode == "" {
			// Right password, second step still to come: not a failure.
			writeJSON(writer, http.StatusUnauthorized, map[string]any{"error": "enter the code from your authenticator app", "totpRequired": true})
			return
		}
		codeStep, codeMatched := auth.MatchTOTPStep(account.TOTPSecret, signInInput.TOTPCode, currentTime)
		if !codeMatched {
			handlers.failSignIn(writer, request, account, "TOTP code is wrong", true)
			return
		}
		stepAccepted, err := handlers.store.UseTOTPStep(ctx, account.ID, codeStep)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "cannot save sign-in")
			return
		}
		if !stepAccepted {
			handlers.failSignIn(writer, request, account, "TOTP code was already used: wait for the next code", true)
			return
		}
	}

	sessionToken := auth.RandomToken(32)
	expiresAt := currentTime.Add(sessionLifetime)
	if err := handlers.store.ResetFailedLogins(ctx, account.ID); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save sign-in")
		return
	}
	if err := handlers.store.CreateSession(ctx, hashSessionToken(sessionToken), account.ID, currentTime, expiresAt); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save sign-in")
		return
	}
	http.SetCookie(writer, handlers.sessionCookie(sessionToken, expiresAt))
	writeJSON(writer, http.StatusOK, map[string]string{"username": account.Username})
}

// failSignIn counts a failed attempt towards the lockout and answers 401.
func (handlers authHandlers) failSignIn(writer http.ResponseWriter, request *http.Request, account store.Account, message string, totpRequired bool) {
	if err := handlers.store.RecordFailedLogin(request.Context(), account.ID, maxFailedLogins, now().Add(loginLockoutTime)); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save sign-in")
		return
	}
	payload := map[string]any{"error": message}
	if totpRequired {
		payload["totpRequired"] = true
	}
	writeJSON(writer, http.StatusUnauthorized, payload)
}

// signOut ends the session server-side and clears the cookie. It succeeds even without a session.
func (handlers authHandlers) signOut(writer http.ResponseWriter, request *http.Request) {
	if sessionCookie, err := request.Cookie(sessionCookieName); err == nil {
		if err := handlers.store.DeleteSession(request.Context(), hashSessionToken(sessionCookie.Value)); err != nil {
			writeError(writer, http.StatusInternalServerError, "cannot end session")
			return
		}
	}
	expiredCookie := handlers.sessionCookie("", time.Unix(0, 0))
	expiredCookie.MaxAge = -1
	http.SetCookie(writer, expiredCookie)
	writer.WriteHeader(http.StatusNoContent)
}

// requireSession runs next with the signed-in username, or answers 401.
func (handlers authHandlers) requireSession(next func(http.ResponseWriter, *http.Request, string)) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		sessionCookie, err := request.Cookie(sessionCookieName)
		if err != nil {
			writeError(writer, http.StatusUnauthorized, "not signed in")
			return
		}
		username, err := handlers.store.FindSessionUsername(request.Context(), hashSessionToken(sessionCookie.Value), now())
		if errors.Is(err, store.ErrSessionNotFound) {
			writeError(writer, http.StatusUnauthorized, "session expired: sign in again")
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "cannot read session")
			return
		}
		next(writer, request, username)
	})
}

// sessionCookie is HttpOnly (no JavaScript access) and SameSite=Strict (never sent from other sites).
func (handlers authHandlers) sessionCookie(value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     handlers.cookiePath,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   handlers.secureCookie,
		SameSite: http.SameSiteStrictMode,
	}
}

func hashSessionToken(sessionToken string) string {
	tokenHash := sha256.Sum256([]byte(sessionToken))
	return hex.EncodeToString(tokenHash[:])
}
