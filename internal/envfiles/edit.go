package envfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

var (
	ErrConflict     = errors.New("the file changed since you opened it: reload and try again")
	ErrInvalidKey   = errors.New("key must start with a letter or _ and contain only letters, digits, _ . -")
	ErrInvalidValue = errors.New("a value cannot contain a line break")
)

const (
	// historyFolder sits next to the env files; List skips it (it is a folder, and hidden).
	historyFolder      = ".history"
	maxHistoryVersions = 50
	historyTimeLayout  = "20060102T150405.000000000Z"
)

// now is replaced in tests to name history entries at a fixed time.
var now = time.Now

// writeLock allows one save at a time, so two users cannot both pass the version check
// and overwrite each other.
var writeLock sync.Mutex

// Change sets Key to *Value, or removes every line of Key when Value is nil.
type Change struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// Content returns the whole file as text (every value in clear) and its version.
func Content(folder, name string) (content, version string, err error) {
	content, err = readFile(folder, name)
	if err != nil {
		return "", "", err
	}
	return content, contentVersion(content), nil
}

// WriteContent replaces the file with content. baseVersion is the version the user
// started from; if the file changed since then, nothing is written and ErrConflict is returned.
func WriteContent(folder, name, baseVersion, author, content string) (string, error) {
	return update(folder, name, baseVersion, author, func(string) (string, error) {
		return content, nil
	})
}

// ApplyChanges sets or removes variables and keeps every other line (comments, order) as it is.
func ApplyChanges(folder, name, baseVersion, author string, changes []Change) (string, error) {
	return update(folder, name, baseVersion, author, func(current string) (string, error) {
		return applyChanges(current, changes)
	})
}

func contentVersion(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func update(folder, name, baseVersion, author string, edit func(current string) (string, error)) (string, error) {
	if !fileNamePattern.MatchString(name) {
		return "", ErrInvalidName
	}
	writeLock.Lock()
	defer writeLock.Unlock()

	root, err := os.OpenRoot(folder)
	if err != nil {
		return "", err
	}
	defer root.Close()
	currentBytes, err := root.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrFileNotFound
	}
	if err != nil {
		return "", err
	}
	current := string(currentBytes)
	if contentVersion(current) != baseVersion {
		return "", ErrConflict
	}
	updated, err := edit(current)
	if err != nil {
		return "", err
	}
	if updated == current {
		return baseVersion, nil
	}
	// Backup first: if it fails the file is untouched, and once it exists a failed or
	// partial write below can always be undone from it.
	if err := saveHistory(root, name, current, author); err != nil {
		return "", fmt.Errorf("cannot write history: %w", err)
	}
	// In place, not temp file + rename: the file keeps its owner, mode and inode,
	// so a container that bind-mounts this single file still sees the change.
	if err := root.WriteFile(name, []byte(updated), 0o600); err != nil {
		return "", err
	}
	return contentVersion(updated), nil
}

// saveHistory stores content as it was before author's save, as
// .history/<name>/<time>_<author>.env, and keeps the newest maxHistoryVersions.
func saveHistory(root *os.Root, name, content, author string) error {
	historyDirectory := path.Join(historyFolder, name)
	if err := root.MkdirAll(historyDirectory, 0o700); err != nil {
		return err
	}
	entryName := now().UTC().Format(historyTimeLayout) + "_" + author + ".env"
	if err := root.WriteFile(path.Join(historyDirectory, entryName), []byte(content), 0o600); err != nil {
		return err
	}
	// Pruning is best effort: if it fails, a few extra old versions stay on disk.
	entries, _ := fs.ReadDir(root.FS(), historyDirectory)
	for len(entries) > maxHistoryVersions {
		// Names start with the time, so ReadDir's name order is oldest first.
		_ = root.Remove(path.Join(historyDirectory, entries[0].Name()))
		entries = entries[1:]
	}
	return nil
}

func applyChanges(current string, changes []Change) (string, error) {
	lineEnding := ""
	if strings.Contains(current, "\r\n") {
		lineEnding = "\r" // lines are split on "\n", so each keeps its "\r"
	}
	lines := strings.Split(current, "\n")
	for _, change := range changes {
		if !keyPattern.MatchString(change.Key) {
			return "", fmt.Errorf("%w: %q", ErrInvalidKey, change.Key)
		}
		if change.Value != nil && strings.ContainsAny(*change.Value, "\r\n") {
			return "", ErrInvalidValue
		}
		updatedLines := make([]string, 0, len(lines)+1)
		lastLineOfKey := -1
		for _, line := range lines {
			if key, _, kind := parseLine(line); kind == assignmentLine && key == change.Key {
				if change.Value == nil {
					// Every line, or an overridden duplicate would become the active value.
					continue
				}
				lastLineOfKey = len(updatedLines)
			}
			updatedLines = append(updatedLines, line)
		}
		switch {
		case change.Value == nil && len(updatedLines) == len(lines):
			return "", fmt.Errorf("%w: %s", ErrVariableNotFound, change.Key)
		case change.Value != nil && lastLineOfKey >= 0:
			// Only the line Compose uses; "export " is kept so the file stays source-able.
			exportPrefix := ""
			if strings.HasPrefix(strings.TrimSpace(updatedLines[lastLineOfKey]), "export ") {
				exportPrefix = "export "
			}
			updatedLines[lastLineOfKey] = exportPrefix + change.Key + "=" + *change.Value + lineEnding
		case change.Value != nil:
			updatedLines = appendLine(updatedLines, change.Key+"="+*change.Value+lineEnding)
		}
		lines = updatedLines
	}
	return strings.Join(lines, "\n"), nil
}

// appendLine adds line at the end, before the final newline when the file has one.
func appendLine(lines []string, line string) []string {
	lastIndex := len(lines) - 1
	if strings.TrimSpace(lines[lastIndex]) == "" {
		return append(lines[:lastIndex], line, lines[lastIndex])
	}
	return append(lines, line)
}
