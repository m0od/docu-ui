package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/m0od/docu-ui/internal/envfiles"
)

const folderNotSet = "choose the env folder first"

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
		writeEnvFileError(writer, err)
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
		writeEnvFileError(writer, err)
		return
	}
	// Audit trail: who looked at which secret (never the value itself).
	slog.Info("env value revealed", "user", username, "file", fileName, "key", key)
	writeJSON(writer, http.StatusOK, map[string]string{"value": value})
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

func writeEnvFileError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, envfiles.ErrInvalidName):
		writeError(writer, http.StatusBadRequest, err.Error())
	case errors.Is(err, envfiles.ErrFileNotFound), errors.Is(err, envfiles.ErrVariableNotFound):
		writeError(writer, http.StatusNotFound, err.Error())
	default:
		slog.Error("read env file", "err", err)
		writeError(writer, http.StatusInternalServerError, "cannot read the env file")
	}
}
