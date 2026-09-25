package dococd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeDocoCD records the last request and answers with statusCode and responseBody.
func fakeDocoCD(tester *testing.T, statusCode int, responseBody string) (*httptest.Server, *http.Request) {
	tester.Helper()
	lastRequest := &http.Request{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*lastRequest = *request.Clone(context.Background())
		writer.WriteHeader(statusCode)
		_, _ = writer.Write([]byte(responseBody))
	}))
	tester.Cleanup(server.Close)
	return server, lastRequest
}

func TestRecreateOneService(tester *testing.T) {
	server, lastRequest := fakeDocoCD(tester, http.StatusOK, `{"content":"service recreated"}`)
	// A trailing slash in the saved URL must not produce "//v1".
	if err := Recreate(context.Background(), server.URL+"/", "secret-key", "textiq-dev", "keycloak"); err != nil {
		tester.Fatal(err)
	}
	if lastRequest.Method != http.MethodPost || lastRequest.URL.Path != "/v1/api/project/textiq-dev/recreate" ||
		lastRequest.URL.Query().Get("service") != "keycloak" || lastRequest.Header.Get("x-api-key") != "secret-key" {
		tester.Fatalf("got %s %s key=%q", lastRequest.Method, lastRequest.URL, lastRequest.Header.Get("x-api-key"))
	}
}

// Without a service, Doco-CD recreates every service of the project.
func TestRecreateWholeProject(tester *testing.T) {
	server, lastRequest := fakeDocoCD(tester, http.StatusOK, "")
	if err := Recreate(context.Background(), server.URL, "key", "textiq-dev", ""); err != nil {
		tester.Fatal(err)
	}
	if lastRequest.URL.RawQuery != "" {
		tester.Fatalf("got query %q", lastRequest.URL.RawQuery)
	}
}

// The user fixes the problem from this message, so Doco-CD's own reason must reach them.
func TestRecreateReportsDocoCDError(tester *testing.T) {
	testCases := []struct {
		statusCode      int
		responseBody    string
		expectedMessage string
	}{
		{http.StatusUnauthorized, `{"error":"invalid api key"}`, "doco-cd answered 401: invalid api key"},
		{http.StatusNotFound, `{"error":"project not found: x"}`, "doco-cd answered 404: project not found: x"},
		// JSON without an error field: show it as it came rather than an empty reason.
		{http.StatusInternalServerError, `{"content":"x"}`, `doco-cd answered 500: {"content":"x"}`},
		// A reverse proxy in front of Doco-CD answers in plain text.
		{http.StatusBadGateway, "Bad Gateway\n", "doco-cd answered 502: Bad Gateway"},
	}
	for _, testCase := range testCases {
		server, _ := fakeDocoCD(tester, testCase.statusCode, testCase.responseBody)
		err := Recreate(context.Background(), server.URL, "key", "textiq-dev", "")
		if err == nil || err.Error() != testCase.expectedMessage {
			tester.Errorf("got %v, want %q", err, testCase.expectedMessage)
		}
	}
}

func TestRecreateConnectionErrors(tester *testing.T) {
	if err := Recreate(context.Background(), "http://bad host", "key", "p", ""); err == nil {
		tester.Error("invalid URL must fail")
	}
	server, _ := fakeDocoCD(tester, http.StatusOK, "")
	server.Close()
	if err := Recreate(context.Background(), server.URL, "key", "p", ""); err == nil || !strings.Contains(err.Error(), "connect") {
		tester.Errorf("unreachable Doco-CD: %v", err)
	}
}
