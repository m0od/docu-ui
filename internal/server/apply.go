package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/m0od/docu-ui/internal/compose"
	"github.com/m0od/docu-ui/internal/dococd"
	"github.com/m0od/docu-ui/internal/envfiles"
	"github.com/m0od/docu-ui/internal/store"
	"github.com/m0od/docu-ui/internal/webhook"
)

const (
	docoCDNotSet   = "set up Doco-CD first"
	webhookNotSet  = "set up the shared webhook first, or give this file its own"
	targetNotSet   = "choose where to apply this file first"
	cannotSettings = "cannot read settings"
)

// Names as Docker Compose accepts them; they end up in the Doco-CD URL.
var (
	projectPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	servicePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
)

// docoCD never sends the API key back; the page only needs to know one is saved.
func (handlers envFileHandlers) docoCD(writer http.ResponseWriter, request *http.Request, _ string) {
	settings, err := handlers.store.DocoCD(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"url": settings.URL, "hasApiKey": settings.APIKey != ""})
}

func (handlers envFileHandlers) setDocoCD(writer http.ResponseWriter, request *http.Request, username string) {
	var settingsInput struct {
		URL    string `json:"url"`
		APIKey string `json:"apiKey"`
	}
	if !decodeJSON(writer, request, &settingsInput) {
		return
	}
	if !isHTTPURL(settingsInput.URL) {
		writeError(writer, http.StatusBadRequest, "the Doco-CD URL must look like http://doco-cd:80")
		return
	}
	saved, err := handlers.store.DocoCD(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	// The page never shows the saved key, so an empty field means "keep it".
	if settingsInput.APIKey == "" {
		settingsInput.APIKey = saved.APIKey
	}
	if err := handlers.store.SetDocoCD(request.Context(), store.DocoCD{URL: settingsInput.URL, APIKey: settingsInput.APIKey}); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save settings")
		return
	}
	slog.Info("doco-cd settings changed", "user", username, "url", settingsInput.URL)
	writeJSON(writer, http.StatusOK, map[string]any{"url": settingsInput.URL, "hasApiKey": settingsInput.APIKey != ""})
}

// applyTarget answers {target: null} for a file that was never set up.
func (handlers envFileHandlers) applyTarget(writer http.ResponseWriter, request *http.Request, _ string) {
	target, err := handlers.store.ApplyTarget(request.Context(), request.PathValue("name"))
	if errors.Is(err, store.ErrNoApplyTarget) {
		writeJSON(writer, http.StatusOK, map[string]any{"target": nil, "appliedVersion": ""})
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	writeApplyTarget(writer, target)
}

func (handlers envFileHandlers) setApplyTarget(writer http.ResponseWriter, request *http.Request, username string) {
	var targetInput struct {
		Adapter  string   `json:"adapter"`
		Project  string   `json:"project"`
		Services []string `json:"services"`
		// Only for the webhook adapter; an empty URL means the shared webhook.
		Webhook webhookInput `json:"webhook"`
	}
	if !decodeJSON(writer, request, &targetInput) {
		return
	}
	switch targetInput.Adapter {
	case store.AdapterDocoCD, store.AdapterWebhook, store.AdapterCompose:
	default:
		writeError(writer, http.StatusBadRequest, "adapter must be doco-cd, webhook or compose")
		return
	}
	// A webhook receiver may not care about Compose names; Doco-CD and Compose need the project.
	isOptionalProject := targetInput.Adapter == store.AdapterWebhook && targetInput.Project == ""
	if !isOptionalProject && !projectPattern.MatchString(targetInput.Project) {
		writeError(writer, http.StatusBadRequest, "the project name must be lowercase letters, digits, - or _")
		return
	}
	for _, service := range targetInput.Services {
		if !servicePattern.MatchString(service) {
			writeError(writer, http.StatusBadRequest, "invalid service name: "+service)
			return
		}
	}
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName := request.PathValue("name")
	envFile, err := envfiles.Read(folder, fileName)
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	target, err := handlers.store.ApplyTarget(request.Context(), fileName)
	if errors.Is(err, store.ErrNoApplyTarget) {
		// First setup: assume the containers already run with the file as it is now.
		target.AppliedVersion = envFile.Version
	} else if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	ownWebhook := store.Webhook{} // Doco-CD, or the shared webhook: keep no secrets for this file
	if targetInput.Adapter == store.AdapterWebhook && targetInput.Webhook.URL != "" {
		var problem string
		// An empty secret field keeps the one saved for this file.
		if ownWebhook, problem = targetInput.Webhook.merge(target.Webhook); problem != "" {
			writeError(writer, http.StatusBadRequest, problem)
			return
		}
	}
	target.Adapter = targetInput.Adapter
	target.Project = targetInput.Project
	target.Services = append([]string{}, targetInput.Services...)
	target.Webhook = ownWebhook
	if err := handlers.store.SetApplyTarget(request.Context(), fileName, target); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save settings")
		return
	}
	slog.Info("apply target changed", "user", username, "file", fileName, "adapter", target.Adapter,
		"project", target.Project, "services", target.Services, "own_webhook", target.Webhook.URL)
	writeApplyTarget(writer, target)
}

// apply hands the saved file to the target's adapter, so the services pick it up.
func (handlers envFileHandlers) apply(writer http.ResponseWriter, request *http.Request, username string) {
	var applyInput struct {
		// The version the user looked at; a newer save must be reviewed before it goes live.
		Version string `json:"version"`
	}
	if !decodeJSON(writer, request, &applyInput) {
		return
	}
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName := request.PathValue("name")
	envFile, err := envfiles.Read(folder, fileName)
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	if envFile.Version != applyInput.Version {
		writeEnvFileError(writer, envfiles.ErrConflict, cannotRead)
		return
	}
	target, err := handlers.store.ApplyTarget(request.Context(), fileName)
	if errors.Is(err, store.ErrNoApplyTarget) {
		writeError(writer, http.StatusConflict, targetNotSet)
		return
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	delivery := webhook.Delivery{
		File: fileName, Version: applyInput.Version, Project: target.Project, Services: target.Services,
		User: username, SentAt: time.Now().UTC(),
	}
	if statusCode, err := handlers.runAdapter(request.Context(), target, delivery); err != nil {
		slog.Error("apply failed", "user", username, "file", fileName, "adapter", target.Adapter, "err", err)
		writeError(writer, statusCode, err.Error())
		return
	}
	// If the file was saved again during the recreate, the containers may run a newer version than
	// the one recorded here; the page then still says "not applied", which is the safe mistake.
	target.AppliedVersion = applyInput.Version
	if err := handlers.store.SetApplyTarget(request.Context(), fileName, target); err != nil {
		writeError(writer, http.StatusInternalServerError, "applied, but cannot record it")
		return
	}
	slog.Info("env file applied", "user", username, "file", fileName, "adapter", target.Adapter, "project", target.Project, "services", target.Services)
	writeApplyTarget(writer, target)
}

// runAdapter applies target; on failure it returns the HTTP status to answer with.
func (handlers envFileHandlers) runAdapter(ctx context.Context, target store.ApplyTarget, delivery webhook.Delivery) (int, error) {
	switch target.Adapter {
	case store.AdapterCompose:
		if err := compose.Recreate(ctx, target.Project, target.Services); err != nil {
			return http.StatusBadGateway, err
		}
		return 0, nil
	case store.AdapterWebhook:
		// store.Webhook and webhook.Endpoint have the same fields, so they convert directly.
		endpoint := webhook.Endpoint(target.Webhook)
		if endpoint.URL == "" {
			shared, err := handlers.store.Webhook(ctx)
			if err != nil {
				return http.StatusInternalServerError, errors.New(cannotSettings)
			}
			endpoint = webhook.Endpoint(shared)
		}
		if endpoint.URL == "" {
			return http.StatusConflict, errors.New(webhookNotSet)
		}
		if err := webhook.Send(ctx, endpoint, delivery); err != nil {
			return http.StatusBadGateway, err
		}
		return 0, nil
	}
	settings, err := handlers.store.DocoCD(ctx)
	if err != nil {
		return http.StatusInternalServerError, errors.New(cannotSettings)
	}
	if settings.URL == "" {
		return http.StatusConflict, errors.New(docoCDNotSet)
	}
	services := target.Services
	if len(services) == 0 {
		services = []string{""} // one call for the whole project
	}
	for _, service := range services {
		if err := dococd.Recreate(ctx, settings.URL, settings.APIKey, target.Project, service); err != nil {
			return http.StatusBadGateway, err
		}
	}
	return 0, nil
}

// isHTTPURL accepts absolute http(s) URLs, the only ones the adapters can call.
func isHTTPURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	return err == nil && (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != ""
}

func writeApplyTarget(writer http.ResponseWriter, target store.ApplyTarget) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"target": map[string]any{
			"adapter": target.Adapter, "project": target.Project, "services": target.Services,
			"webhook": webhookView(target.Webhook),
		},
		"appliedVersion": target.AppliedVersion,
	})
}
