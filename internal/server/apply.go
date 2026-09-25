package server

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"

	"github.com/m0od/docu-ui/internal/dococd"
	"github.com/m0od/docu-ui/internal/envfiles"
	"github.com/m0od/docu-ui/internal/store"
)

const (
	docoCDNotSet   = "set up Doco-CD first"
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
	parsedURL, err := url.Parse(settingsInput.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
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
		Project  string   `json:"project"`
		Services []string `json:"services"`
	}
	if !decodeJSON(writer, request, &targetInput) {
		return
	}
	if !projectPattern.MatchString(targetInput.Project) {
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
	target.Project = targetInput.Project
	target.Services = append([]string{}, targetInput.Services...)
	if err := handlers.store.SetApplyTarget(request.Context(), fileName, target); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save settings")
		return
	}
	slog.Info("apply target changed", "user", username, "file", fileName, "project", target.Project, "services", target.Services)
	writeApplyTarget(writer, target)
}

// apply recreates the target's services through Doco-CD, so they pick up the saved file.
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
	settings, err := handlers.store.DocoCD(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, cannotSettings)
		return
	}
	if settings.URL == "" {
		writeError(writer, http.StatusConflict, docoCDNotSet)
		return
	}
	services := target.Services
	if len(services) == 0 {
		services = []string{""} // one call for the whole project
	}
	for _, service := range services {
		if err := dococd.Recreate(request.Context(), settings.URL, settings.APIKey, target.Project, service); err != nil {
			slog.Error("apply failed", "user", username, "file", fileName, "project", target.Project, "service", service, "err", err)
			writeError(writer, http.StatusBadGateway, err.Error())
			return
		}
	}
	// If the file was saved again during the recreate, the containers may run a newer version than
	// the one recorded here; the page then still says "not applied", which is the safe mistake.
	target.AppliedVersion = applyInput.Version
	if err := handlers.store.SetApplyTarget(request.Context(), fileName, target); err != nil {
		writeError(writer, http.StatusInternalServerError, "applied, but cannot record it")
		return
	}
	slog.Info("env file applied", "user", username, "file", fileName, "project", target.Project, "services", target.Services)
	writeApplyTarget(writer, target)
}

func writeApplyTarget(writer http.ResponseWriter, target store.ApplyTarget) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"target":         map[string]any{"project": target.Project, "services": target.Services},
		"appliedVersion": target.AppliedVersion,
	})
}
