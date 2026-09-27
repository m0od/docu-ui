package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/store"
)

const (
	sessionCookieName = "docu_session"
	// OWASP session timeouts: 30 minutes without a request ends a session (idle), and none lives
	// past 8 hours however active (absolute), so a stolen cookie is good for a working day at most.
	sessionIdleTimeout = 30 * time.Minute
	sessionMaxLifetime = 8 * time.Hour
	// Five wrong passwords or TOTP codes lock the account for 15 minutes: slow enough to stop
	// guessing, short enough that a locked-out admin just waits.
	maxFailedLogins  = 5
	loginLockoutTime = 15 * time.Minute
	invalidLogin     = "invalid username or password"
	// anonymousUser is who history and logs name when sign-in is off.
	anonymousUser = "anonymous"
)

// SignInOffWarning is logged at start and whenever sign-in is turned off.
const SignInOffWarning = "sign-in is off: anyone who reaches Docu-UI can read and change every env file"

// signInOffKey marks a request let through because sign-in is off, not because of a session.
type signInOffKey struct{}

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
	// allowedHosts are the host names (lower case, no port) accepted while sign-in is off,
	// on top of localhost and IP addresses.
	allowedHosts map[string]bool
}

func (handlers authHandlers) register(routes *http.ServeMux) {
	routes.HandleFunc("POST /api/auth/login", handlers.signIn)
	routes.HandleFunc("POST /api/auth/logout", handlers.signOut)
	routes.Handle("GET /api/auth/me", handlers.requireSession(func(writer http.ResponseWriter, request *http.Request, username string) {
		writeJSON(writer, http.StatusOK, map[string]any{"username": username, "signIn": !isSignInOff(request)})
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
	// A locked account answers like a wrong password, after the same slow check: a different status
	// or a faster answer would tell a stranger that the username exists.
	if account.LockedUntil.After(currentTime) {
		_, _ = auth.VerifyPassword(signInInput.Password, account.PasswordHash)
		writeError(writer, http.StatusUnauthorized, invalidLogin)
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
		codeResult, err := checkSecondFactor(ctx, handlers.store, account, signInInput.TOTPCode)
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "cannot save sign-in")
			return
		}
		if codeResult != secondFactorAccepted {
			handlers.failSignIn(writer, request, account, codeResult.problem(), true)
			return
		}
	}

	sessionToken := auth.RandomToken(32)
	expiresAt := currentTime.Add(sessionMaxLifetime)
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
	handlers.clearSessionCookie(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (handlers authHandlers) clearSessionCookie(writer http.ResponseWriter) {
	expiredCookie := handlers.sessionCookie("", time.Unix(0, 0))
	expiredCookie.MaxAge = -1
	http.SetCookie(writer, expiredCookie)
}

// requireSession runs next with the signed-in username, or answers 401.
// With sign-in off, every request goes through as anonymousUser.
func (handlers authHandlers) requireSession(next func(http.ResponseWriter, *http.Request, string)) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		signInOff, err := handlers.store.SignInOff(request.Context())
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "cannot read accounts")
			return
		}
		if signInOff {
			if hostName := requestHostName(request.Host); !handlers.hostAllowed(hostName) {
				// Only a page from another site, whose name now resolves to Docu-UI (DNS rebinding),
				// reaches here: without sign-in, nothing else would stop it from reading every value.
				slog.Warn("request refused: host not allowed while sign-in is off", "host", hostName)
				writeError(writer, http.StatusForbidden, "host "+hostName+" is not allowed while sign-in is off: add it to DOCU_ALLOWED_HOSTS")
				return
			}
			next(writer, request.WithContext(context.WithValue(request.Context(), signInOffKey{}, true)), anonymousUser)
			return
		}
		sessionCookie, err := request.Cookie(sessionCookieName)
		if err != nil {
			writeError(writer, http.StatusUnauthorized, "not signed in")
			return
		}
		username, err := handlers.store.UseSession(request.Context(), hashSessionToken(sessionCookie.Value), now(), sessionIdleTimeout)
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

// requestHostName turns a Host header into the name DOCU_ALLOWED_HOSTS lists: lower case,
// without port, brackets or trailing dot.
func requestHostName(hostHeader string) string {
	hostName, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		hostName = hostHeader // no port
	}
	return strings.ToLower(strings.TrimSuffix(strings.Trim(hostName, "[]"), "."))
}

// hostAllowed accepts localhost and IP addresses, which another site cannot make its own name
// point to, and the host names listed in DOCU_ALLOWED_HOSTS.
func (handlers authHandlers) hostAllowed(hostName string) bool {
	if hostName == "localhost" || strings.HasSuffix(hostName, ".localhost") || net.ParseIP(hostName) != nil {
		return true
	}
	return handlers.allowedHosts[hostName]
}

func isSignInOff(request *http.Request) bool {
	return request.Context().Value(signInOffKey{}) != nil
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
