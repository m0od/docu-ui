// Package server wires the HTTP routes: health check and the single-page UI.
package server

import (
	"bytes"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Config holds what the HTTP layer needs from the environment.
type Config struct {
	// BasePath is the sub-path the app is served under behind a gateway, e.g. "/docu-ui". Empty means "/".
	BasePath string
	// Store backs setup, sign-in and sessions.
	Store Store
	// SetupToken must be typed on the setup page; it is printed to the log only while no account exists.
	SetupToken string
	// InsecureCookie drops the Secure flag from the session cookie so sign-in works over plain HTTP.
	// Only for a trusted network; normally TLS is on at the gateway or in Docu-UI itself.
	InsecureCookie bool
}

// New returns the root handler.
func New(config Config, uiFiles fs.FS) (http.Handler, error) {
	basePath := NormalizeBasePath(config.BasePath)

	indexPage, err := fs.ReadFile(uiFiles, "index.html")
	if err != nil {
		return nil, err
	}
	// Relative asset URLs in the build resolve against this, so the UI works under any sub-path.
	indexPage = bytes.Replace(indexPage, []byte("<head>"),
		[]byte(`<head><base href="`+html.EscapeString(basePath+"/")+`">`), 1)

	appRoutes := http.NewServeMux()
	appRoutes.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = writer.Write([]byte("ok"))
	})
	setupHandlers{accounts: config.Store, setupToken: config.SetupToken}.register(appRoutes)
	authHandlers{store: config.Store, cookiePath: basePath + "/", secureCookie: !config.InsecureCookie}.register(appRoutes)
	// Unknown API paths get a JSON 404, not the UI page.
	appRoutes.HandleFunc("GET /api/", func(writer http.ResponseWriter, _ *http.Request) {
		writeError(writer, http.StatusNotFound, "not found")
	})
	appRoutes.Handle("GET /", spa(uiFiles, indexPage))

	if basePath == "" {
		return appRoutes, nil
	}
	gatewayRoutes := http.NewServeMux()
	gatewayRoutes.Handle(basePath+"/", http.StripPrefix(basePath, appRoutes))
	// "/docu-ui" without the trailing slash would break relative URLs
	gatewayRoutes.Handle(basePath, http.RedirectHandler(basePath+"/", http.StatusMovedPermanently))
	return gatewayRoutes, nil
}

// NormalizeBasePath turns "", "/", "docu-ui", "/docu-ui/" into "" or "/docu-ui".
func NormalizeBasePath(rawPath string) string {
	trimmedPath := strings.Trim(strings.TrimSpace(rawPath), "/")
	if trimmedPath == "" {
		return ""
	}
	return "/" + trimmedPath
}

// spa serves built files as-is and falls back to index.html for client-side routes.
func spa(uiFiles fs.FS, indexPage []byte) http.Handler {
	fileServer := http.FileServerFS(uiFiles)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fileName := strings.TrimPrefix(path.Clean("/"+request.URL.Path), "/")
		if fileName != "" && fileName != "index.html" {
			if fileInfo, err := fs.Stat(uiFiles, fileName); err == nil && !fileInfo.IsDir() {
				fileServer.ServeHTTP(writer, request)
				return
			}
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Cache-Control", "no-cache")
		_, _ = writer.Write(indexPage)
	})
}
