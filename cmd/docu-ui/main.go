// Command docu-ui serves the web UI for editing Docker Compose env files.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0od/docu-ui/internal/auth"
	"github.com/m0od/docu-ui/internal/server"
	"github.com/m0od/docu-ui/internal/store"
	"github.com/m0od/docu-ui/web"
)

// startupConfig is what Docu-UI reads from its environment variables.
type startupConfig struct {
	listenAddress, dataDirectory string
	tlsCertFile, tlsKeyFile      string
	basePath                     string
	insecureCookie               bool
	allowedHosts                 []string
}

// loadConfig reads the settings through lookupEnv (os.Getenv outside tests).
func loadConfig(lookupEnv func(key string) string) startupConfig {
	withDefault := func(key, fallback string) string {
		if value := lookupEnv(key); value != "" {
			return value
		}
		return fallback
	}
	return startupConfig{
		listenAddress: withDefault("DOCU_ADDR", ":8080"),
		dataDirectory: withDefault("DOCU_DATA_DIR", "/data"),
		tlsCertFile:   lookupEnv("DOCU_TLS_CERT"),
		tlsKeyFile:    lookupEnv("DOCU_TLS_KEY"),
		basePath:      lookupEnv("DOCU_BASE_PATH"),
		// Only for plain-HTTP setups on a trusted network; browsers drop Secure cookies over http.
		insecureCookie: lookupEnv("DOCU_INSECURE_COOKIE") == "true",
		allowedHosts:   strings.Split(lookupEnv("DOCU_ALLOWED_HOSTS"), ","),
	}
}

// servesTLS: with a cert, Docu-UI serves HTTPS itself (standalone). Without one it expects a gateway
// to terminate TLS. A key alone also picks HTTPS, so the missing cert fails loudly at start.
func (startup startupConfig) servesTLS() bool {
	return startup.tlsCertFile != "" || startup.tlsKeyFile != ""
}

func main() {
	startup := loadConfig(os.Getenv)

	accountStore, err := store.Open(filepath.Join(startup.dataDirectory, "docu-ui.db"))
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
	signInOff, err := accountStore.SignInOff(context.Background())
	if err != nil {
		fatal("read accounts", err)
	}
	if signInOff {
		slog.Warn(server.SignInOffWarning)
	}

	handler, err := server.New(server.Config{
		BasePath:       startup.basePath,
		Store:          accountStore,
		SetupToken:     setupToken,
		InsecureCookie: startup.insecureCookie,
		AllowedHosts:   startup.allowedHosts,
	}, web.Dist())
	if err != nil {
		fatal("init server", err)
	}
	httpServer := &http.Server{Addr: startup.listenAddress, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", startup.listenAddress, "base_path", server.NormalizeBasePath(startup.basePath),
		"tls", startup.tlsCertFile != "")
	if startup.servesTLS() {
		err = httpServer.ListenAndServeTLS(startup.tlsCertFile, startup.tlsKeyFile)
	} else {
		err = httpServer.ListenAndServe()
	}
	fatal("server stopped", err)
}

func fatal(message string, err error) {
	slog.Error(message, "err", err)
	os.Exit(1)
}
