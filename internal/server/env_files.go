package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/m0od/docu-ui/internal/envfiles"
)

const (
	folderNotSet = "choose the env folder first"
	cannotRead   = "cannot read the env file"
	cannotSave   = "cannot save the env file"
)

type envFileHandlers struct {
	store          Store
	requireSession func(next func(http.ResponseWriter, *http.Request, string)) http.Handler
}

func (handlers envFileHandlers) register(routes *http.ServeMux) {
	routes.Handle("GET /api/settings/env-folder", handlers.requireSession(handlers.envFolder))
	routes.Handle("PUT /api/settings/env-folder", handlers.requireSession(handlers.setEnvFolder))
	routes.Handle("GET /api/env-files", handlers.requireSession(handlers.listFiles))
	routes.Handle("GET /api/env-files/{name}", handlers.requireSession(handlers.readFile))
	routes.Handle("GET /api/env-files/{name}/variables/{key}", handlers.requireSession(handlers.revealValue))
	routes.Handle("PATCH /api/env-files/{name}/variables", handlers.requireSession(handlers.changeVariables))
	routes.Handle("GET /api/env-files/{name}/content", handlers.requireSession(handlers.readContent))
	routes.Handle("PUT /api/env-files/{name}/content", handlers.requireSession(handlers.writeContent))
	routes.Handle("GET /api/env-files/{name}/history", handlers.requireSession(handlers.listHistory))
	routes.Handle("GET /api/env-files/{name}/history/{id}", handlers.requireSession(handlers.readHistory))
	routes.Handle("POST /api/env-files/{name}/history/{id}/restore", handlers.requireSession(handlers.restoreHistory))
	routes.Handle("GET /api/settings/doco-cd", handlers.requireSession(handlers.docoCD))
	routes.Handle("PUT /api/settings/doco-cd", handlers.requireSession(handlers.setDocoCD))
	routes.Handle("GET /api/settings/webhook", handlers.requireSession(handlers.sharedWebhook))
	routes.Handle("PUT /api/settings/webhook", handlers.requireSession(handlers.setSharedWebhook))
	routes.Handle("GET /api/env-files/{name}/apply-target", handlers.requireSession(handlers.applyTarget))
	routes.Handle("PUT /api/env-files/{name}/apply-target", handlers.requireSession(handlers.setApplyTarget))
	routes.Handle("POST /api/env-files/{name}/apply", handlers.requireSession(handlers.apply))
}

func (handlers envFileHandlers) envFolder(writer http.ResponseWriter, request *http.Request, _ string) {
	folder, err := handlers.store.EnvFolder(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot read settings")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"folder": folder})
}

func (handlers envFileHandlers) setEnvFolder(writer http.ResponseWriter, request *http.Request, username string) {
	var folderInput struct {
		Folder string `json:"folder"`
	}
	if !decodeJSON(writer, request, &folderInput) {
		return
	}
	// Checked before saving, so a typo or an unmounted path never replaces a working folder.
	if err := envfiles.CheckFolder(folderInput.Folder); err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := handlers.store.SetEnvFolder(request.Context(), folderInput.Folder); err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot save settings")
		return
	}
	slog.Info("env folder changed", "user", username, "folder", folderInput.Folder)
	writeJSON(writer, http.StatusOK, map[string]string{"folder": folderInput.Folder})
}

func (handlers envFileHandlers) listFiles(writer http.ResponseWriter, request *http.Request, _ string) {
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileNames, err := envfiles.List(folder)
	if err != nil {
		// Usually the volume is no longer mounted; say so, the user can pick another folder.
		writeError(writer, http.StatusInternalServerError, "cannot read the env folder: "+err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"folder": folder, "files": fileNames})
}

func (handlers envFileHandlers) readFile(writer http.ResponseWriter, request *http.Request, _ string) {
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	envFile, err := envfiles.Read(folder, request.PathValue("name"))
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	writeJSON(writer, http.StatusOK, envFile)
}

func (handlers envFileHandlers) revealValue(writer http.ResponseWriter, request *http.Request, username string) {
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName, key := request.PathValue("name"), request.PathValue("key")
	value, err := envfiles.Value(folder, fileName, key)
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	// Audit trail: who looked at which secret (never the value itself).
	slog.Info("env value revealed", "user", username, "file", fileName, "key", key)
	writeJSON(writer, http.StatusOK, map[string]string{"value": value})
}

// readContent returns the whole file, every value in clear, for the text editor.
func (handlers envFileHandlers) readContent(writer http.ResponseWriter, request *http.Request, username string) {
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName := request.PathValue("name")
	content, version, err := envfiles.Content(folder, fileName)
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	slog.Info("env file opened as text", "user", username, "file", fileName)
	writeJSON(writer, http.StatusOK, map[string]string{"content": content, "version": version})
}

func (handlers envFileHandlers) writeContent(writer http.ResponseWriter, request *http.Request, username string) {
	var contentInput struct {
		BaseVersion string `json:"baseVersion"`
		Content     string `json:"content"`
	}
	if !decodeJSON(writer, request, &contentInput) {
		return
	}
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName := request.PathValue("name")
	version, err := envfiles.WriteContent(folder, fileName, contentInput.BaseVersion, username, contentInput.Content)
	if err != nil {
		writeEnvFileError(writer, err, cannotSave)
		return
	}
	slog.Info("env file saved as text", "user", username, "file", fileName)
	writeJSON(writer, http.StatusOK, map[string]string{"version": version})
}

func (handlers envFileHandlers) changeVariables(writer http.ResponseWriter, request *http.Request, username string) {
	var changesInput struct {
		BaseVersion string            `json:"baseVersion"`
		Changes     []envfiles.Change `json:"changes"`
	}
	if !decodeJSON(writer, request, &changesInput) {
		return
	}
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName := request.PathValue("name")
	version, err := envfiles.ApplyChanges(folder, fileName, changesInput.BaseVersion, username, changesInput.Changes)
	if err != nil {
		writeEnvFileError(writer, err, cannotSave)
		return
	}
	changedKeys := make([]string, 0, len(changesInput.Changes))
	for _, change := range changesInput.Changes {
		changedKeys = append(changedKeys, change.Key)
	}
	// Keys only: the log must never hold a secret value.
	slog.Info("env variables changed", "user", username, "file", fileName, "keys", changedKeys)
	writeJSON(writer, http.StatusOK, map[string]string{"version": version})
}

func (handlers envFileHandlers) listHistory(writer http.ResponseWriter, request *http.Request, _ string) {
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	entries, err := envfiles.History(folder, request.PathValue("name"))
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"entries": entries})
}

// readHistory returns one old version, every value in clear, for the diff view.
func (handlers envFileHandlers) readHistory(writer http.ResponseWriter, request *http.Request, username string) {
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName, entryID := request.PathValue("name"), request.PathValue("id")
	content, err := envfiles.HistoryContent(folder, fileName, entryID)
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	slog.Info("env file history opened", "user", username, "file", fileName, "version", entryID)
	writeJSON(writer, http.StatusOK, map[string]string{"content": content})
}

// restoreHistory writes an old version back. The content is read here, not sent by the browser,
// so the audit line names exactly what was restored. The restore itself is backed up like any save.
func (handlers envFileHandlers) restoreHistory(writer http.ResponseWriter, request *http.Request, username string) {
	var restoreInput struct {
		BaseVersion string `json:"baseVersion"`
	}
	if !decodeJSON(writer, request, &restoreInput) {
		return
	}
	folder, ok := handlers.folder(writer, request)
	if !ok {
		return
	}
	fileName, entryID := request.PathValue("name"), request.PathValue("id")
	content, err := envfiles.HistoryContent(folder, fileName, entryID)
	if err != nil {
		writeEnvFileError(writer, err, cannotRead)
		return
	}
	version, err := envfiles.WriteContent(folder, fileName, restoreInput.BaseVersion, username, content)
	if err != nil {
		writeEnvFileError(writer, err, cannotSave)
		return
	}
	slog.Info("env file restored", "user", username, "file", fileName, "version", entryID)
	writeJSON(writer, http.StatusOK, map[string]string{"version": version})
}

// folder returns the saved env folder, or answers 409/500 and returns false.
func (handlers envFileHandlers) folder(writer http.ResponseWriter, request *http.Request) (string, bool) {
	folder, err := handlers.store.EnvFolder(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "cannot read settings")
		return "", false
	}
	if folder == "" {
		writeError(writer, http.StatusConflict, folderNotSet)
		return "", false
	}
	return folder, true
}

// writeEnvFileError maps envfiles errors to HTTP. Other errors (permission denied, disk full)
// are shown after failureMessage, since the fix is usually on the host (mount, owner).
func writeEnvFileError(writer http.ResponseWriter, err error, failureMessage string) {
	switch {
	case errors.Is(err, envfiles.ErrInvalidName), errors.Is(err, envfiles.ErrInvalidKey), errors.Is(err, envfiles.ErrInvalidValue):
		writeError(writer, http.StatusBadRequest, err.Error())
	case errors.Is(err, envfiles.ErrFileNotFound), errors.Is(err, envfiles.ErrVariableNotFound),
		errors.Is(err, envfiles.ErrHistoryNotFound):
		writeError(writer, http.StatusNotFound, err.Error())
	case errors.Is(err, envfiles.ErrConflict):
		writeError(writer, http.StatusConflict, err.Error())
	default:
		slog.Error(failureMessage, "err", err)
		writeError(writer, http.StatusInternalServerError, failureMessage+": "+err.Error())
	}
}
