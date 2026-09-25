package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

var testUI = fstest.MapFS{
	"index.html":    {Data: []byte("<html><head><title>x</title></head></html>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

func get(tester *testing.T, handler http.Handler, target string) *http.Response {
	tester.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder.Result()
}

func readBody(tester *testing.T, response *http.Response) string {
	tester.Helper()
	content, err := io.ReadAll(response.Body)
	if err != nil {
		tester.Fatal(err)
	}
	return string(content)
}

func mustNew(tester *testing.T, basePath string) http.Handler {
	tester.Helper()
	handler, err := New(Config{BasePath: basePath}, testUI)
	if err != nil {
		tester.Fatal(err)
	}
	return handler
}

// Gateways (ALB, Docker healthcheck) probe this; it must answer without a session.
func TestHealthz(tester *testing.T) {
	response := get(tester, mustNew(tester, ""), "/healthz")
	if response.StatusCode != http.StatusOK || readBody(tester, response) != "ok" {
		tester.Fatalf("got %d", response.StatusCode)
	}
}

func TestServesBuiltAsset(tester *testing.T) {
	response := get(tester, mustNew(tester, ""), "/assets/app.js")
	if response.StatusCode != http.StatusOK || readBody(tester, response) != "console.log(1)" {
		tester.Fatalf("got %d", response.StatusCode)
	}
}

// A browser refresh on a client-side route must get the app, not a 404.
func TestUnknownPathFallsBackToIndex(tester *testing.T) {
	for _, requestPath := range []string{"/", "/index.html", "/services/iam", "/assets"} {
		response := get(tester, mustNew(tester, ""), requestPath)
		content := readBody(tester, response)
		if response.StatusCode != http.StatusOK || !strings.Contains(content, "<title>x</title>") {
			tester.Fatalf("%s: got %d %q", requestPath, response.StatusCode, content)
		}
		if response.Header.Get("Cache-Control") != "no-cache" {
			tester.Fatalf("%s: index must not be cached, or a redeploy keeps serving old asset names", requestPath)
		}
	}
}

// Relative asset URLs only resolve under a sub-path if the page carries the right <base href>.
func TestIndexCarriesBaseHref(tester *testing.T) {
	expectedBaseTags := map[string]string{"": `<base href="/">`, "/docu-ui/": `<base href="/docu-ui/">`}
	for basePath, expectedTag := range expectedBaseTags {
		response := get(tester, mustNew(tester, basePath), NormalizeBasePath(basePath)+"/")
		if content := readBody(tester, response); !strings.Contains(content, expectedTag) {
			tester.Fatalf("base %q: %q missing in %q", basePath, expectedTag, content)
		}
	}
}

func TestBasePathRouting(tester *testing.T) {
	handler := mustNew(tester, "docu-ui")
	if response := get(tester, handler, "/docu-ui/healthz"); response.StatusCode != http.StatusOK {
		tester.Fatalf("healthz under base: %d", response.StatusCode)
	}
	if response := get(tester, handler, "/docu-ui/assets/app.js"); readBody(tester, response) != "console.log(1)" {
		tester.Fatal("asset under base not served")
	}
	// Without the slash, relative URLs would resolve against "/" instead of the sub-path.
	redirect := get(tester, handler, "/docu-ui")
	if redirect.StatusCode != http.StatusMovedPermanently || redirect.Header.Get("Location") != "/docu-ui/" {
		tester.Fatalf("got %d %q", redirect.StatusCode, redirect.Header.Get("Location"))
	}
	// Paths outside the sub-path belong to other apps on the gateway.
	if response := get(tester, handler, "/healthz"); response.StatusCode != http.StatusNotFound {
		tester.Fatalf("outside base: %d", response.StatusCode)
	}
}

func TestNormalizeBasePath(tester *testing.T) {
	expectedPaths := map[string]string{"": "", "/": "", " /a/ ": "/a", "a": "/a", "/a/b/": "/a/b"}
	for rawPath, expectedPath := range expectedPaths {
		if normalizedPath := NormalizeBasePath(rawPath); normalizedPath != expectedPath {
			tester.Errorf("%q: got %q want %q", rawPath, normalizedPath, expectedPath)
		}
	}
}

// A build without index.html is broken; fail at startup, not on the first request.
func TestNewFailsWithoutIndex(tester *testing.T) {
	if _, err := New(Config{}, fstest.MapFS{}); err == nil {
		tester.Fatal("expected error")
	}
}
