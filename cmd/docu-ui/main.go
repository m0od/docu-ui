// Command docu-ui serves the web UI for editing Docker Compose env files.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/m0od/docu-ui/internal/server"
	"github.com/m0od/docu-ui/web"
)

func main() {
	listenAddress := getenv("DOCU_ADDR", ":8080")
	handler, err := server.New(server.Config{BasePath: os.Getenv("DOCU_BASE_PATH")}, web.Dist())
	if err != nil {
		slog.Error("init server", "err", err)
		os.Exit(1)
	}
	httpServer := &http.Server{Addr: listenAddress, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("listening", "addr", listenAddress, "base_path", server.NormalizeBasePath(os.Getenv("DOCU_BASE_PATH")))
	if err := httpServer.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
