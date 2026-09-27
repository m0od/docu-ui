package server

import (
	"log/slog"
	"net/http"
	"net/url"
	"regexp"

	"github.com/m0od/docu-ui/internal/store"
)

// An HTTP header name (RFC 9110 token).
var headerNamePattern = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

// webhookInput is a webhook as the page sends it. Secrets are write-only: empty keeps the saved one.
type webhookInput struct {
	URL         string `json:"url"`
	Secret      string `json:"secret"`
	HeaderName  string `json:"headerName"`
	HeaderValue string `json:"headerValue"`
}

// merge validates the input and fills empty secret fields from saved; problem is "" when valid.
// Saved secrets are only kept while the URL stays on the same host, so nobody who can edit
// settings can point the webhook at their own server and receive them.
func (input webhookInput) merge(saved store.Webhook) (merged store.Webhook, problem string) {
	if !isHTTPURL(input.URL) {
		return store.Webhook{}, "the webhook URL must look like https://ci.example.com/hook"
	}
	if input.HeaderName != "" && !headerNamePattern.MatchString(input.HeaderName) {
		return store.Webhook{}, "invalid header name: " + input.HeaderName
	}
	merged = store.Webhook{URL: input.URL, Secret: input.Secret, HeaderName: input.HeaderName, HeaderValue: input.HeaderValue}
	if merged.Secret == "" {
		merged.Secret = saved.Secret
	}
	if merged.HeaderName == "" {
		merged.HeaderValue = "" // no header, so no value to keep
	} else if merged.HeaderValue == "" {
		merged.HeaderValue = saved.HeaderValue
	}
	keepsSavedSecret := (input.Secret == "" && merged.Secret != "") || (input.HeaderValue == "" && merged.HeaderValue != "")
	if keepsSavedSecret && !sameOrigin(saved.URL, input.URL) {
		return store.Webhook{}, newHostNeedsSecrets
	}
	return merged, ""
}

const newHostNeedsSecrets = "the URL points to another host: enter the secret and header value again"

// sameOrigin says whether two URLs have the same scheme, host and port.
func sameOrigin(savedURL, newURL string) bool {
	parsedSaved, savedErr := url.Parse(savedURL)
	parsedNew, newErr := url.Parse(newURL)
	return savedErr == nil && newErr == nil && parsedSaved.Scheme == parsedNew.Scheme && parsedSaved.Host == parsedNew.Host
}

// webhookView is a webhook as the page may see it: whether secrets are set, never their values.
func webhookView(hook store.Webhook) map[string]any {
	return map[string]any{
		"url": hook.URL, "hasSecret": hook.Secret != "",
		"headerName": hook.HeaderName, "hasHeaderValue": hook.HeaderValue != "",
	}
}

func (handlers envFileHandlers) sharedWebhook(writer http.ResponseWriter, request *http.Request, _ string) {
	shared, err := handlers.store.Webhook(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	writeJSON(writer, http.StatusOK, webhookView(shared))
}

func (handlers envFileHandlers) setSharedWebhook(writer http.ResponseWriter, request *http.Request, username string) {
	var input webhookInput
	if !decodeJSON(writer, request, &input) {
		return
	}
	saved, err := handlers.store.Webhook(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	merged, problem := input.merge(saved)
	if problem != "" {
		writeError(writer, http.StatusBadRequest, problem)
		return
	}
	if err := handlers.store.SetWebhook(request.Context(), merged); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save settings")
		return
	}
	slog.Info("shared webhook changed", "user", username, "url", merged.URL, "header", merged.HeaderName)
	writeJSON(writer, http.StatusOK, webhookView(merged))
}
