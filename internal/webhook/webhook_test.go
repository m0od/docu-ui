package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type receivedRequest struct {
	header http.Header
	body   []byte
}

// fakeReceiver records what it got and answers with statusCode and answer.
func fakeReceiver(tester *testing.T, statusCode int, answer string) (*httptest.Server, *receivedRequest) {
	tester.Helper()
	received := &receivedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received.header = request.Header.Clone()
		received.body, _ = io.ReadAll(request.Body)
		writer.WriteHeader(statusCode)
		_, _ = writer.Write([]byte(answer))
	}))
	tester.Cleanup(server.Close)
	return server, received
}

var testDelivery = Delivery{
	File: "keycloak.env", Version: "v2", Project: "textiq-dev", Services: []string{"keycloak"},
	User: "admin", SentAt: time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC),
}

// A GitHub repository_dispatch body: GitHub Actions can be triggered with no receiver in between.
func TestSendPostsADispatchShapedBody(tester *testing.T) {
	server, received := fakeReceiver(tester, http.StatusNoContent, "")
	if err := Send(context.Background(), Endpoint{URL: server.URL}, testDelivery); err != nil {
		tester.Fatal(err)
	}
	var sentBody map[string]any
	if err := json.Unmarshal(received.body, &sentBody); err != nil {
		tester.Fatal(err)
	}
	payload, _ := sentBody["client_payload"].(map[string]any)
	if sentBody["event_type"] != "docu-ui.apply" || payload["file"] != "keycloak.env" || payload["project"] != "textiq-dev" ||
		payload["sentAt"] != "2026-09-25T08:00:00Z" || received.header.Get("Content-Type") != "application/json" {
		tester.Fatalf("got %s %v", received.body, received.header)
	}
	// Unsigned when no secret is set, rather than signed with an empty key that proves nothing.
	if received.header.Get(SignatureHeader) != "" {
		tester.Fatal("signed without a secret")
	}
}

// The receiver recomputes the HMAC of the exact bytes it got; a match proves Docu-UI sent them.
func TestSendSignsTheBody(tester *testing.T) {
	server, received := fakeReceiver(tester, http.StatusOK, "")
	endpoint := Endpoint{URL: server.URL, Secret: "shared-secret", HeaderName: "Authorization", HeaderValue: "Bearer token"}
	if err := Send(context.Background(), endpoint, testDelivery); err != nil {
		tester.Fatal(err)
	}
	signature := received.header.Get(SignatureHeader)
	if signature != Signature("shared-secret", received.body) || signature == Signature("other-secret", received.body) ||
		!strings.HasPrefix(signature, "sha256=") {
		tester.Fatalf("signature %q", signature)
	}
	if received.header.Get("Authorization") != "Bearer token" {
		tester.Fatalf("header %v", received.header)
	}
}

// A custom header named like one Docu-UI sets must not break the request or fake the signature.
func TestCustomHeaderCannotReplaceOwnHeaders(tester *testing.T) {
	server, received := fakeReceiver(tester, http.StatusOK, "")
	endpoint := Endpoint{URL: server.URL, Secret: "s", HeaderName: SignatureHeader, HeaderValue: "sha256=forged"}
	if err := Send(context.Background(), endpoint, testDelivery); err != nil {
		tester.Fatal(err)
	}
	if received.header.Get(SignatureHeader) != Signature("s", received.body) {
		tester.Fatalf("signature %q", received.header.Get(SignatureHeader))
	}
}

// Documented example: HMAC-SHA256 of "hello" with key "secret", so receivers can check their code.
func TestSignatureKnownValue(tester *testing.T) {
	expected := "sha256=88aab3ede8d3adf94d26ab90d3bafd4a2083070c3bcce9c014ee04a443847c0b"
	if signature := Signature("secret", []byte("hello")); signature != expected {
		tester.Fatalf("got %s", signature)
	}
}

func TestSendReportsReceiverError(tester *testing.T) {
	server, _ := fakeReceiver(tester, http.StatusUnauthorized, "Bad credentials\n")
	err := Send(context.Background(), Endpoint{URL: server.URL}, testDelivery)
	if err == nil || err.Error() != "webhook answered 401: Bad credentials" {
		tester.Fatalf("got %v", err)
	}
}

func TestSendConnectionErrors(tester *testing.T) {
	if err := Send(context.Background(), Endpoint{URL: "http://bad host"}, testDelivery); err == nil {
		tester.Error("invalid URL must fail")
	}
	server, _ := fakeReceiver(tester, http.StatusOK, "")
	server.Close()
	if err := Send(context.Background(), Endpoint{URL: server.URL}, testDelivery); err == nil {
		tester.Error("unreachable receiver must fail")
	}
}
