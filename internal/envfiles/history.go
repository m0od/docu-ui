package envfiles

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"time"
)

var ErrHistoryNotFound = errors.New("history version not found")

// historyEntryPattern matches names written by saveHistory: <time>_<author>.env.
var historyEntryPattern = regexp.MustCompile(`^(\d{8}T\d{6}\.\d{9}Z)_([A-Za-z0-9._-]+)\.env$`)

// HistoryEntry is the content of a file as it was just before SavedBy saved it at SavedAt.
type HistoryEntry struct {
	ID      string    `json:"id"`
	SavedAt time.Time `json:"savedAt"`
	SavedBy string    `json:"savedBy"`
}

// History lists the stored versions of folder/name, newest first.
func History(folder, name string) ([]HistoryEntry, error) {
	if !fileNamePattern.MatchString(name) {
		return nil, ErrInvalidName
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), path.Join(historyFolder, name))
	if errors.Is(err, fs.ErrNotExist) {
		return []HistoryEntry{}, nil // never saved through Docu-UI
	}
	if err != nil {
		return nil, err
	}
	historyEntries := []HistoryEntry{}
	for _, entry := range slices.Backward(entries) {
		parts := historyEntryPattern.FindStringSubmatch(entry.Name())
		if parts == nil {
			continue // not written by Docu-UI
		}
		// The pattern guarantees a valid layout, except impossible dates like month 13.
		savedAt, err := time.Parse(historyTimeLayout, parts[1])
		if err != nil {
			continue
		}
		historyEntries = append(historyEntries, HistoryEntry{ID: entry.Name(), SavedAt: savedAt, SavedBy: parts[2]})
	}
	return historyEntries, nil
}

// HistoryContent returns one stored version of folder/name.
func HistoryContent(folder, name, entryID string) (string, error) {
	if !fileNamePattern.MatchString(name) {
		return "", ErrInvalidName
	}
	// The ID comes from the URL; the pattern keeps it a plain file name inside the history folder.
	if !historyEntryPattern.MatchString(entryID) {
		return "", ErrHistoryNotFound
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return "", err
	}
	defer root.Close()
	content, err := root.ReadFile(path.Join(historyFolder, name, entryID))
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrHistoryNotFound
	}
	return string(content), err
}
