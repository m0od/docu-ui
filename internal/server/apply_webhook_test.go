package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/m0od/docu-ui/internal/webhook"
)

type receivedDelivery struct {
	signature, authorization string
	body                     []byte
}

// fakeReceiver stands in for a CI/CD webhook and records every delivery.
func fakeReceiver(tester *testing.T, statusCode int) (*httptest.Server, *[]receivedDelivery) {
	tester.Helper()
	deliveries := &[]receivedDelivery{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*deliveries = append(*deliveries, receivedDelivery{request.Header.Get(webhook.SignatureHeader), request.Header.Get("Authorization"), body})
		writer.WriteHeader(statusCode)
	}))
	tester.Cleanup(server.Close)
	return server, deliveries
}

func webhookTargetBody(project string, ownWebhook map[string]any) string {
	return editorBody(map[string]any{"adapter": "webhook", "project": project, "services": []string{"keycloak"}, "webhook": ownWebhook})
}

func TestSharedWebhookSettingsHideSecrets(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	fresh := sendJSON(handler, http.MethodGet, "/api/settings/webhook", "", sessionCookie)
	if responseField(tester, fresh, "url") != "" || responseField(tester, fresh, "hasSecret") != false {
		tester.Fatalf("fresh: %s", fresh.Body.String())
	}

	saved := sendJSON(handler, http.MethodPut, "/api/settings/webhook", editorBody(map[string]any{
		"url": "https://ci/hook", "secret": "hmac-secret", "headerName": "Authorization", "headerValue": "Bearer ci-token",
	}), sessionCookie)
	if saved.Code != http.StatusOK || strings.Contains(saved.Body.String(), "hmac-secret") || strings.Contains(saved.Body.String(), "ci-token") ||
		responseField(tester, saved, "hasSecret") != true || responseField(tester, saved, "hasHeaderValue") != true {
		tester.Fatalf("save: %d %s", saved.Code, saved.Body.String())
	}
	// The page cannot show secrets, so empty fields keep them.
	kept := sendJSON(handler, http.MethodPut, "/api/settings/webhook",
		editorBody(map[string]any{"url": "https://ci/other", "headerName": "Authorization"}), sessionCookie)
	if responseField(tester, kept, "hasSecret") != true || responseField(tester, kept, "hasHeaderValue") != true {
		tester.Fatalf("keep: %s", kept.Body.String())
	}
	// Removing the header name removes its value too; a later header must not inherit an old token.
	dropped := sendJSON(handler, http.MethodPut, "/api/settings/webhook", editorBody(map[string]any{"url": "https://ci/other"}), sessionCookie)
	if responseField(tester, dropped, "headerName") != "" || responseField(tester, dropped, "hasHeaderValue") != false ||
		responseField(tester, dropped, "hasSecret") != true {
		tester.Fatalf("drop header: %s", dropped.Body.String())
	}
}

func TestSharedWebhookRejectsBadInput(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	for _, body := range []string{"not json", `{"url":"ci/hook"}`, `{"url":"https://ci/hook","headerName":"Bad Header"}`} {
		if response := sendJSON(handler, http.MethodPut, "/api/settings/webhook", body, sessionCookie); response.Code != http.StatusBadRequest {
			tester.Errorf("%s: got %d", body, response.Code)
		}
	}
}

// Through the shared webhook: the receiver gets a signed, value-free delivery it can verify.
func TestApplyThroughSharedWebhook(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	receiver, deliveries := fakeReceiver(tester, http.StatusNoContent)
	envFolderWith(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPut, "/api/settings/webhook", editorBody(map[string]any{"url": receiver.URL, "secret": "hmac-secret"}), sessionCookie)
	// A webhook receiver may not need a Compose project.
	if response := sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/apply-target", webhookTargetBody("", nil), sessionCookie); response.Code != http.StatusOK {
		tester.Fatalf("target: %d %s", response.Code, response.Body.String())
	}
	version := openForEdit(tester, handler, sessionCookie)

	applied := sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", editorBody(map[string]any{"version": version}), sessionCookie)
	if applied.Code != http.StatusOK || responseField(tester, applied, "appliedVersion") != version || len(*deliveries) != 1 {
		tester.Fatalf("apply: %d %s, %d deliveries", applied.Code, applied.Body.String(), len(*deliveries))
	}
	delivery := (*deliveries)[0]
	if delivery.signature != webhook.Signature("hmac-secret", delivery.body) {
		tester.Fatalf("signature %q", delivery.signature)
	}
	var sentBody struct {
		EventType     string           `json:"event_type"`
		ClientPayload webhook.Delivery `json:"client_payload"`
	}
	json.Unmarshal(delivery.body, &sentBody)
	if sentBody.EventType != webhook.EventType || sentBody.ClientPayload.File != "keycloak.env" || sentBody.ClientPayload.Version != version ||
		sentBody.ClientPayload.User != "admin" || strings.Join(sentBody.ClientPayload.Services, ",") != "keycloak" {
		tester.Fatalf("body %s", delivery.body)
	}
	// Never a value: the receiver learns what changed, not the secrets in it.
	if strings.Contains(string(delivery.body), "s3cret") || strings.Contains(string(delivery.body), "postgres") {
		tester.Fatalf("values leaked: %s", delivery.body)
	}
}

// A file's own webhook wins over the shared one, keeps its secret across edits, and is dropped on switching to Doco-CD.
func TestApplyThroughOwnWebhook(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	sharedReceiver, sharedDeliveries := fakeReceiver(tester, http.StatusOK)
	ownReceiver, ownDeliveries := fakeReceiver(tester, http.StatusOK)
	envFolderWith(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPut, "/api/settings/webhook", editorBody(map[string]any{"url": sharedReceiver.URL}), sessionCookie)
	targetURL := "/api/env-files/keycloak.env/apply-target"
	ownWebhook := map[string]any{"url": ownReceiver.URL, "secret": "own-secret", "headerName": "Authorization", "headerValue": "Bearer own"}
	target := sendJSON(handler, http.MethodPut, targetURL, webhookTargetBody("textiq-dev", ownWebhook), sessionCookie)
	if strings.Contains(target.Body.String(), "own-secret") || strings.Contains(target.Body.String(), "Bearer own") {
		tester.Fatalf("secrets returned: %s", target.Body.String())
	}
	// Edited again without retyping the secrets: they stay.
	sendJSON(handler, http.MethodPut, targetURL, webhookTargetBody("textiq-dev", map[string]any{"url": ownReceiver.URL, "headerName": "Authorization"}), sessionCookie)

	version := openForEdit(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", editorBody(map[string]any{"version": version}), sessionCookie)
	if len(*sharedDeliveries) != 0 || len(*ownDeliveries) != 1 {
		tester.Fatalf("shared %d, own %d", len(*sharedDeliveries), len(*ownDeliveries))
	}
	if delivery := (*ownDeliveries)[0]; delivery.signature != webhook.Signature("own-secret", delivery.body) || delivery.authorization != "Bearer own" {
		tester.Fatalf("delivery %+v", delivery)
	}

	switched := sendJSON(handler, http.MethodPut, targetURL, editorBody(map[string]any{"adapter": "doco-cd", "project": "textiq-dev", "webhook": ownWebhook}), sessionCookie)
	view, _ := responseField(tester, switched, "target").(map[string]any)["webhook"].(map[string]any)
	if view["url"] != "" || view["hasSecret"] != false {
		tester.Fatalf("switched: %s", switched.Body.String())
	}
}

func TestApplyWebhookErrors(tester *testing.T) {
	handler, sessionCookie := signedIn(tester, openAdminStore(tester, ""))
	failingReceiver, _ := fakeReceiver(tester, http.StatusInternalServerError)
	envFolderWith(tester, handler, sessionCookie)
	targetURL := "/api/env-files/keycloak.env/apply-target"
	testCases := []struct {
		body         string
		expectedCode int
	}{
		{editorBody(map[string]any{"adapter": "ftp", "project": "textiq-dev"}), http.StatusBadRequest},
		{webhookTargetBody("TextIQ", nil), http.StatusBadRequest},
		{webhookTargetBody("", map[string]any{"url": "not a url"}), http.StatusBadRequest},
	}
	for _, testCase := range testCases {
		if response := sendJSON(handler, http.MethodPut, targetURL, testCase.body, sessionCookie); response.Code != testCase.expectedCode {
			tester.Errorf("%s: got %d %s", testCase.body, response.Code, response.Body.String())
		}
	}

	sendJSON(handler, http.MethodPut, targetURL, webhookTargetBody("", nil), sessionCookie)
	applyBody := editorBody(map[string]any{"version": openForEdit(tester, handler, sessionCookie)})
	response := sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody, sessionCookie)
	if response.Code != http.StatusConflict || responseField(tester, response, "error") != webhookNotSet {
		tester.Fatalf("no webhook: %d %s", response.Code, response.Body.String())
	}
	sendJSON(handler, http.MethodPut, "/api/settings/webhook", editorBody(map[string]any{"url": failingReceiver.URL}), sessionCookie)
	response = sendJSON(handler, http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody, sessionCookie)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), "webhook answered 500") {
		tester.Fatalf("receiver down: %d %s", response.Code, response.Body.String())
	}
}

func TestWebhookRoutesReportStoreFailures(tester *testing.T) {
	adminStore := openAdminStore(tester, "")
	handler, sessionCookie := signedIn(tester, adminStore)
	envFolderWith(tester, handler, sessionCookie)
	sendJSON(handler, http.MethodPut, "/api/env-files/keycloak.env/apply-target", webhookTargetBody("", nil), sessionCookie)
	applyBody := editorBody(map[string]any{"version": openForEdit(tester, handler, sessionCookie)})
	settingsBody := editorBody(map[string]any{"url": "https://ci/hook"})
	testCases := []struct {
		failingMethod, method, target, body string
	}{
		{"Webhook", http.MethodGet, "/api/settings/webhook", ""},
		{"Webhook", http.MethodPut, "/api/settings/webhook", settingsBody},
		{"SetWebhook", http.MethodPut, "/api/settings/webhook", settingsBody},
		{"Webhook", http.MethodPost, "/api/env-files/keycloak.env/apply", applyBody},
	}
	for _, testCase := range testCases {
		failingHandler, failingCookie := signedIn(tester, failingStore{Store: adminStore, failingMethod: testCase.failingMethod})
		if response := sendJSON(failingHandler, testCase.method, testCase.target, testCase.body, failingCookie); response.Code != http.StatusInternalServerError {
			tester.Errorf("%s %s with %s down: got %d", testCase.method, testCase.target, testCase.failingMethod, response.Code)
		}
	}
}
