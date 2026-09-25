// Package envfiles reads Docker Compose env files (name.env) from one folder.
// The folder is passed on every call, so changing it on the web applies at once.
package envfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	ErrInvalidName      = errors.New("file name must look like name.env")
	ErrFileNotFound     = errors.New("env file not found")
	ErrVariableNotFound = errors.New("variable not found")
)

var (
	// No slashes and no leading dot, so a name can never point outside the folder.
	fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.env$`)
	keyPattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
)

// Variable is one KEY=value line. Its value is only sent when the user asks to see it.
type Variable struct {
	Key  string `json:"key"`
	Line int    `json:"line"`
	// Overridden: a later line sets the same key, and Compose uses that one.
	Overridden bool `json:"overridden"`
}

// File is an env file without its values.
type File struct {
	Name string `json:"name"`
	// Version identifies this exact content; a save must send it back (see WriteContent).
	Version   string     `json:"version"`
	Variables []Variable `json:"variables"`
	// InvalidLines are neither comments nor KEY=value; Compose would reject the file.
	InvalidLines []int `json:"invalidLines"`
}

// CheckFolder reports whether folder can be used as the env folder.
func CheckFolder(folder string) error {
	if !filepath.IsAbs(folder) {
		return errors.New("folder must be an absolute path, e.g. /host/env")
	}
	if _, err := os.ReadDir(folder); err != nil {
		return fmt.Errorf("cannot read folder: %w", err)
	}
	return nil
}

// List returns the names of the env files in folder, sorted.
func List(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	fileNames := []string{}
	for _, entry := range entries {
		// Symlinks and sub-folders are skipped: only plain files that live in the folder itself.
		if entry.Type().IsRegular() && fileNamePattern.MatchString(entry.Name()) {
			fileNames = append(fileNames, entry.Name())
		}
	}
	return fileNames, nil // os.ReadDir already sorts by name
}

// Read returns the variables in folder/name.
func Read(folder, name string) (File, error) {
	content, err := readFile(folder, name)
	if err != nil {
		return File{}, err
	}
	assignments, invalidLines := parse(content)
	lastLineOfKey := map[string]int{}
	for _, assignment := range assignments {
		lastLineOfKey[assignment.key] = assignment.line
	}
	variables := make([]Variable, 0, len(assignments))
	for _, assignment := range assignments {
		variables = append(variables, Variable{
			Key:        assignment.key,
			Line:       assignment.line,
			Overridden: lastLineOfKey[assignment.key] != assignment.line,
		})
	}
	return File{Name: name, Version: contentVersion(content), Variables: variables, InvalidLines: invalidLines}, nil
}

// Value returns the value of key as written in the file (quotes kept).
// With duplicates it is the last one, the one Compose uses.
func Value(folder, name, key string) (string, error) {
	content, err := readFile(folder, name)
	if err != nil {
		return "", err
	}
	assignments, _ := parse(content)
	for _, assignment := range slices.Backward(assignments) {
		if assignment.key == key {
			return assignment.value, nil
		}
	}
	return "", ErrVariableNotFound
}

func readFile(folder, name string) (string, error) {
	if !fileNamePattern.MatchString(name) {
		return "", ErrInvalidName
	}
	// os.Root refuses symlinks that lead out of the folder.
	root, err := os.OpenRoot(folder)
	if err != nil {
		return "", err
	}
	defer root.Close()
	content, err := root.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrFileNotFound
	}
	return string(content), err
}

type assignment struct {
	key   string
	value string
	line  int
}

// parse reads KEY=value lines. Blank lines and # comments are skipped; "export KEY=value" is allowed.
func parse(content string) (assignments []assignment, invalidLines []int) {
	invalidLines = []int{}
	for index, rawLine := range strings.Split(content, "\n") {
		key, value, kind := parseLine(rawLine)
		switch kind {
		case assignmentLine:
			assignments = append(assignments, assignment{key: key, value: value, line: index + 1})
		case invalidLine:
			invalidLines = append(invalidLines, index+1)
		}
	}
	return assignments, invalidLines
}

type lineKind int

const (
	skippedLine lineKind = iota // blank or comment
	assignmentLine
	invalidLine
)

func parseLine(rawLine string) (key, value string, kind lineKind) {
	line := strings.TrimSpace(rawLine)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", skippedLine
	}
	key, value, found := strings.Cut(strings.TrimPrefix(line, "export "), "=")
	key = strings.TrimSpace(key)
	if !found || !keyPattern.MatchString(key) {
		return "", "", invalidLine
	}
	return key, strings.TrimSpace(value), assignmentLine
}
