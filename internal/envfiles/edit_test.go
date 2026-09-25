package envfiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stringPointer(value string) *string { return &value }

// editableFolder returns a folder holding keycloak.env with content, and its version.
func editableFolder(tester *testing.T, content string) (string, string) {
	tester.Helper()
	folder := tester.TempDir()
	writeFile(tester, filepath.Join(folder, "keycloak.env"), content)
	return folder, contentVersion(content)
}

func readBack(tester *testing.T, folder string) string {
	tester.Helper()
	content, err := os.ReadFile(filepath.Join(folder, "keycloak.env"))
	if err != nil {
		tester.Fatal(err)
	}
	return string(content)
}

func historyEntries(tester *testing.T, folder string) []os.DirEntry {
	tester.Helper()
	entries, _ := os.ReadDir(filepath.Join(folder, historyFolder, "keycloak.env"))
	return entries
}

func TestContentAndVersion(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	content, contentVersion, err := Content(folder, "keycloak.env")
	if err != nil || content != "A=1\n" || contentVersion != version {
		tester.Fatalf("got %q %q %v", content, contentVersion, err)
	}
	if _, _, err := Content(folder, "../x.env"); !errors.Is(err, ErrInvalidName) {
		tester.Fatalf("bad name: %v", err)
	}
}

func TestWriteContentSavesAndBacksUpThePreviousVersion(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	savedAt := time.Date(2026, 9, 25, 8, 30, 0, 0, time.UTC)
	now = func() time.Time { return savedAt }
	tester.Cleanup(func() { now = time.Now })

	newVersion, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n")
	if err != nil || newVersion != contentVersion("A=2\n") || readBack(tester, folder) != "A=2\n" {
		tester.Fatalf("got %q %v, file %q", newVersion, err, readBack(tester, folder))
	}
	// The backup holds what the file was before admin's save, named by time and author.
	backup, err := os.ReadFile(filepath.Join(folder, historyFolder, "keycloak.env", "20260925T083000.000000000Z_admin.env"))
	if err != nil || string(backup) != "A=1\n" {
		tester.Fatalf("backup %q %v", backup, err)
	}
}

// Two people open the same file; the second save must not silently erase the first.
func TestWriteRefusesAStaleVersion(tester *testing.T) {
	folder, openedVersion := editableFolder(tester, "A=1\n")
	if _, err := WriteContent(folder, "keycloak.env", openedVersion, "alice", "A=2\n"); err != nil {
		tester.Fatal(err)
	}
	if _, err := WriteContent(folder, "keycloak.env", openedVersion, "bob", "A=3\n"); !errors.Is(err, ErrConflict) {
		tester.Fatalf("got %v", err)
	}
	if readBack(tester, folder) != "A=2\n" {
		tester.Fatal("stale save overwrote the file")
	}
}

// Saving without a change must not fill the history with identical copies.
func TestSaveWithoutChangeWritesNothing(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	newVersion, err := WriteContent(folder, "keycloak.env", version, "admin", "A=1\n")
	if err != nil || newVersion != version || len(historyEntries(tester, folder)) != 0 {
		tester.Fatalf("got %q %v, %d history entries", newVersion, err, len(historyEntries(tester, folder)))
	}
}

// Other tools and people own the file; a save must not change who may read it.
func TestWriteKeepsFileMode(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	filePath := filepath.Join(folder, "keycloak.env")
	os.Chmod(filePath, 0o640)
	if _, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n"); err != nil {
		tester.Fatal(err)
	}
	if info, _ := os.Stat(filePath); info.Mode().Perm() != 0o640 {
		tester.Fatalf("mode changed to %v", info.Mode().Perm())
	}
}

func TestHistoryKeepsNewestVersions(tester *testing.T) {
	folder, version := editableFolder(tester, "A=0\n")
	for saveNumber := 1; saveNumber <= maxHistoryVersions+2; saveNumber++ {
		content := "A=" + strings.Repeat("x", saveNumber) + "\n"
		var err error
		if version, err = WriteContent(folder, "keycloak.env", version, "admin", content); err != nil {
			tester.Fatal(err)
		}
	}
	entries := historyEntries(tester, folder)
	if len(entries) != maxHistoryVersions {
		tester.Fatalf("got %d entries", len(entries))
	}
	// The two oldest backups ("A=0", "A=x") are the ones removed.
	oldestKept, _ := os.ReadFile(filepath.Join(folder, historyFolder, "keycloak.env", entries[0].Name()))
	if string(oldestKept) != "A=xx\n" {
		tester.Fatalf("oldest kept %q", oldestKept)
	}
}

func TestWriteErrors(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	if _, err := WriteContent(folder, "../x.env", version, "admin", ""); !errors.Is(err, ErrInvalidName) {
		tester.Errorf("bad name: %v", err)
	}
	if _, err := WriteContent(folder, "missing.env", version, "admin", ""); !errors.Is(err, ErrFileNotFound) {
		tester.Errorf("missing file: %v", err)
	}
	if _, err := WriteContent(filepath.Join(folder, "gone"), "keycloak.env", version, "admin", ""); err == nil {
		tester.Error("missing folder accepted")
	}
	os.Symlink("/etc/hosts", filepath.Join(folder, "escape.env"))
	if _, err := WriteContent(folder, "escape.env", version, "admin", ""); err == nil || errors.Is(err, ErrFileNotFound) {
		tester.Errorf("symlink out of folder: %v", err)
	}
}

// Docu-UI runs as a non-root user; a folder it may not write must fail loudly, file untouched.
func TestWriteFailsWhenHistoryCannotBeWritten(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	os.Chmod(folder, 0o500)
	tester.Cleanup(func() { os.Chmod(folder, 0o700) })
	if _, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n"); err == nil || !strings.Contains(err.Error(), "history") {
		tester.Fatalf("got %v", err)
	}
	if readBack(tester, folder) != "A=1\n" {
		tester.Fatal("file changed although the backup failed")
	}

	os.Chmod(folder, 0o700)
	// The history folder exists but cannot take a new entry.
	os.MkdirAll(filepath.Join(folder, historyFolder, "keycloak.env"), 0o700)
	os.Chmod(filepath.Join(folder, historyFolder, "keycloak.env"), 0o500)
	tester.Cleanup(func() { os.Chmod(filepath.Join(folder, historyFolder, "keycloak.env"), 0o700) })
	if _, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n"); err == nil {
		tester.Fatal("read-only history folder accepted")
	}
}

func TestWriteFailsOnReadOnlyFile(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	os.Chmod(filepath.Join(folder, "keycloak.env"), 0o400)
	if _, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n"); err == nil {
		tester.Fatal("read-only file accepted")
	}
}

func TestWriteFailsOnUnreadableFile(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	os.Chmod(filepath.Join(folder, "keycloak.env"), 0o200)
	if _, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n"); err == nil || errors.Is(err, ErrFileNotFound) {
		tester.Fatalf("got %v", err)
	}
}

func TestApplyChanges(tester *testing.T) {
	original := "# Keycloak\nKC_DB=postgres\nexport KC_HOST=a\nKC_DB=mysql\nOLD=1\n"
	testCases := []struct {
		name     string
		changes  []Change
		expected string
	}{
		{
			// Only the active (last) line changes; the comment and order stay.
			name:     "set existing key",
			changes:  []Change{{Key: "KC_DB", Value: stringPointer("mariadb")}},
			expected: "# Keycloak\nKC_DB=postgres\nexport KC_HOST=a\nKC_DB=mariadb\nOLD=1\n",
		},
		{
			name:     "keep export prefix",
			changes:  []Change{{Key: "KC_HOST", Value: stringPointer("b")}},
			expected: "# Keycloak\nKC_DB=postgres\nexport KC_HOST=b\nKC_DB=mysql\nOLD=1\n",
		},
		{
			name:     "add new key before the final newline",
			changes:  []Change{{Key: "NEW", Value: stringPointer("")}},
			expected: original + "NEW=\n",
		},
		{
			// Removing only the last line would bring the overridden one back to life.
			name:     "remove every line of a key",
			changes:  []Change{{Key: "KC_DB"}},
			expected: "# Keycloak\nexport KC_HOST=a\nOLD=1\n",
		},
		{
			name:     "several changes in order",
			changes:  []Change{{Key: "OLD"}, {Key: "NEW", Value: stringPointer("2")}},
			expected: "# Keycloak\nKC_DB=postgres\nexport KC_HOST=a\nKC_DB=mysql\nNEW=2\n",
		},
	}
	for _, testCase := range testCases {
		updated, err := applyChanges(original, testCase.changes)
		if err != nil || updated != testCase.expected {
			tester.Errorf("%s: got %q %v\nwant %q", testCase.name, updated, err, testCase.expected)
		}
	}
}

func TestApplyChangesFileWithoutFinalNewline(tester *testing.T) {
	if updated, _ := applyChanges("A=1", []Change{{Key: "B", Value: stringPointer("2")}}); updated != "A=1\nB=2" {
		tester.Fatalf("got %q", updated)
	}
}

// Files edited on Windows keep CRLF line endings on every line, new ones included.
func TestApplyChangesKeepsCRLF(tester *testing.T) {
	updated, _ := applyChanges("A=1\r\n", []Change{{Key: "A", Value: stringPointer("2")}, {Key: "B", Value: stringPointer("3")}})
	if updated != "A=2\r\nB=3\r\n" {
		tester.Fatalf("got %q", updated)
	}
}

func TestApplyChangesRejectsBadInput(tester *testing.T) {
	if _, err := applyChanges("A=1\n", []Change{{Key: "1BAD", Value: stringPointer("x")}}); !errors.Is(err, ErrInvalidKey) {
		tester.Errorf("bad key: %v", err)
	}
	// A line break in a value would inject a second variable.
	if _, err := applyChanges("A=1\n", []Change{{Key: "A", Value: stringPointer("x\nADMIN=true")}}); !errors.Is(err, ErrInvalidValue) {
		tester.Errorf("line break: %v", err)
	}
	if _, err := applyChanges("A=1\n", []Change{{Key: "MISSING"}}); !errors.Is(err, ErrVariableNotFound) {
		tester.Errorf("remove missing: %v", err)
	}
}

func TestApplyChangesWritesTheFile(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	if _, err := ApplyChanges(folder, "keycloak.env", version, "admin", []Change{{Key: "A", Value: stringPointer("2")}}); err != nil {
		tester.Fatal(err)
	}
	if readBack(tester, folder) != "A=2\n" {
		tester.Fatalf("got %q", readBack(tester, folder))
	}
	// A rejected change must leave the file and the history alone.
	if _, err := ApplyChanges(folder, "keycloak.env", contentVersion("A=2\n"), "admin", []Change{{Key: "1BAD"}}); !errors.Is(err, ErrInvalidKey) {
		tester.Fatalf("got %v", err)
	}
	if len(historyEntries(tester, folder)) != 1 {
		tester.Fatal("rejected change wrote history")
	}
}
