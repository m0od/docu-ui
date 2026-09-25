// Package webhook tells a CI/CD system that an env file should be applied.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// EventType names the event; GitHub Actions matches it in `on: repository_dispatch: types:`.
	EventType = "docu-ui.apply"
	// SignatureHeader carries "sha256=<hex HMAC-SHA256 of the body>", like GitHub's X-Hub-Signature-256.
	SignatureHeader = "X-Docu-UI-Signature"
)

// Receivers usually queue a job and answer at once; a slow one should not hang the page.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// Endpoint is where and how to send.
type Endpoint struct {
	URL         string
	Secret      string // empty: no signature
	HeaderName  string // empty: no extra header
	HeaderValue string
}

// Delivery says what to apply. It never holds env values.
type Delivery struct {
	File     string    `json:"file"`
	Version  string    `json:"version"`
	Project  string    `json:"project"`
	Services []string  `json:"services"`
	User     string    `json:"user"`
	SentAt   time.Time `json:"sentAt"`
}

// body has the shape of a GitHub repository_dispatch request, so the URL can be
// https://api.github.com/repos/{owner}/{repo}/dispatches; other receivers read it as plain JSON.
type body struct {
	EventType     string   `json:"event_type"`
	ClientPayload Delivery `json:"client_payload"`
}

// Send posts delivery to endpoint and succeeds on any 2xx answer.
func Send(ctx context.Context, endpoint Endpoint, delivery Delivery) error {
	content, _ := json.Marshal(body{EventType: EventType, ClientPayload: delivery}) // plain strings and a time: cannot fail
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.URL, bytes.NewReader(content))
	if err != nil {
		return err
	}
	// The user's header goes first, so it cannot replace the content type or the signature.
	if endpoint.HeaderName != "" {
		request.Header.Set(endpoint.HeaderName, endpoint.HeaderValue)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "Docu-UI")
	if endpoint.Secret != "" {
		request.Header.Set(SignatureHeader, Signature(endpoint.Secret, content))
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 == 2 {
		return nil
	}
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 300))
	return fmt.Errorf("webhook answered %d: %s", response.StatusCode, strings.TrimSpace(string(answer)))
}

// Signature is the SignatureHeader value for content; receivers compute it the same way to verify.
func Signature(secret string, content []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(content)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
