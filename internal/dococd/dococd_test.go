package dococd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
	if err := Recreate(context.Background(), server.URL+"/", "secret-key", "shop-dev", "api"); err != nil {
		tester.Fatal(err)
	}
	if lastRequest.Method != http.MethodPost || lastRequest.URL.Path != "/v1/api/project/shop-dev/recreate" ||
		lastRequest.URL.Query().Get("service") != "api" || lastRequest.Header.Get("x-api-key") != "secret-key" {
		tester.Fatalf("got %s %s key=%q", lastRequest.Method, lastRequest.URL, lastRequest.Header.Get("x-api-key"))
	}
}

// Without a service, Doco-CD recreates every service of the project.
func TestRecreateWholeProject(tester *testing.T) {
	server, lastRequest := fakeDocoCD(tester, http.StatusOK, "")
	if err := Recreate(context.Background(), server.URL, "key", "shop-dev", ""); err != nil {
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
		err := Recreate(context.Background(), server.URL, "key", "shop-dev", "")
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

// A redirect would carry the API key to whatever host the answer names, and turn the POST into
// a GET whose 200 looks like a recreate. So Docu-UI stops at the 3xx and tells the user where it points.
func TestRecreateDoesNotFollowRedirects(tester *testing.T) {
	otherHost, otherHostRequest := fakeDocoCD(tester, http.StatusOK, "")
	redirecting := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, otherHost.URL+"/steal", http.StatusMovedPermanently)
	}))
	tester.Cleanup(redirecting.Close)

	err := Recreate(context.Background(), redirecting.URL, "secret-key", "shop-dev", "api")
	want := "doco-cd answered 301, redirecting to " + otherHost.URL + "/steal: use that URL"
	if err == nil || err.Error() != want {
		tester.Fatalf("got %v, want %q", err, want)
	}
	if otherHostRequest.Method != "" {
		tester.Fatalf("the other host got %s %s", otherHostRequest.Method, otherHostRequest.URL)
	}
}

// A Doco-CD that never answers must not keep the apply request open: after the timeout Recreate
// fails, and the file stays "not applied".
func TestRecreateGivesUpOnAHungDocoCD(tester *testing.T) {
	if httpClient.Timeout == 0 {
		tester.Fatal("no timeout: a hung Doco-CD would hang the apply request forever")
	}
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	tester.Cleanup(hung.Close)
	// Runs before Close (cleanups run last-in first-out), which waits for the handler to return.
	tester.Cleanup(func() { close(release) })
	productionTimeout := httpClient.Timeout
	httpClient.Timeout = 100 * time.Millisecond
	tester.Cleanup(func() { httpClient.Timeout = productionTimeout })

	err := Recreate(context.Background(), hung.URL, "key", "shop-dev", "")
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout exceeded") {
		tester.Fatalf("got %v", err)
	}
}
