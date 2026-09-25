// Command docu-ui serves the web UI for editing Docker Compose env files.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/server"
	"github.com/m0od/docu-ui/internal/store"
	"github.com/m0od/docu-ui/web"
)

func main() {
	listenAddress := getenv("DOCU_ADDR", ":8080")
	dataDirectory := getenv("DOCU_DATA_DIR", "/data")
	tlsCertFile, tlsKeyFile := os.Getenv("DOCU_TLS_CERT"), os.Getenv("DOCU_TLS_KEY")

	accountStore, err := store.Open(filepath.Join(dataDirectory, "docu-ui.db"))
	if err != nil {
		fatal("open database", err)
	}
	defer accountStore.Close()

	needsSetup, err := accountStore.NeedsSetup(context.Background())
	if err != nil {
		fatal("read accounts", err)
	}
	var setupToken string
	if needsSetup {
		// New on every start, so a token copied from an old log stops working after a restart.
		setupToken = auth.RandomToken(16)
		slog.Warn("no account yet: open the UI and create the admin with this setup token", "setup_token", setupToken)
	}

	handler, err := server.New(server.Config{
		BasePath:   os.Getenv("DOCU_BASE_PATH"),
		Store:      accountStore,
		SetupToken: setupToken,
		// Only for plain-HTTP setups on a trusted network; browsers drop Secure cookies over http.
		InsecureCookie: os.Getenv("DOCU_INSECURE_COOKIE") == "true",
	}, web.Dist())
	if err != nil {
		fatal("init server", err)
	}
	httpServer := &http.Server{Addr: listenAddress, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", listenAddress, "base_path", server.NormalizeBasePath(os.Getenv("DOCU_BASE_PATH")),
		"tls", tlsCertFile != "")
	// With a cert, Docu-UI serves HTTPS itself (standalone). Without one it expects a gateway to terminate TLS.
	if tlsCertFile != "" || tlsKeyFile != "" {
		err = httpServer.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	} else {
		err = httpServer.ListenAndServe()
	}
	fatal("server stopped", err)
}

func fatal(message string, err error) {
	slog.Error(message, "err", err)
	os.Exit(1)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
