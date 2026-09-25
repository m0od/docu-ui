package envfiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// saveAt makes the next save look like it happened at savedAt.
func saveAt(tester *testing.T, savedAt time.Time) {
	tester.Helper()
	now = func() time.Time { return savedAt }
	tester.Cleanup(func() { now = time.Now })
}

func TestHistoryListsSavesNewestFirst(tester *testing.T) {
	folder, version := editableFolder(tester, "A=1\n")
	firstSave := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	saveAt(tester, firstSave)
	version, err := WriteContent(folder, "keycloak.env", version, "admin", "A=2\n")
	if err != nil {
		tester.Fatal(err)
	}
	secondSave := firstSave.Add(time.Hour)
	saveAt(tester, secondSave)
	if _, err := WriteContent(folder, "keycloak.env", version, "ops.user", "A=3\n"); err != nil {
		tester.Fatal(err)
	}

	entries, err := History(folder, "keycloak.env")
	if err != nil || len(entries) != 2 {
		tester.Fatalf("got %v %v", entries, err)
	}
	// The newest save is what people look for first; the author may contain dots.
	if !entries[0].SavedAt.Equal(secondSave) || entries[0].SavedBy != "ops.user" ||
		!entries[1].SavedAt.Equal(firstSave) || entries[1].SavedBy != "admin" {
		tester.Fatalf("got %+v", entries)
	}
	// Each entry holds the file as it was just before that save.
	for entryIndex, expectedContent := range []string{"A=2\n", "A=1\n"} {
		content, err := HistoryContent(folder, "keycloak.env", entries[entryIndex].ID)
		if err != nil || content != expectedContent {
			tester.Fatalf("entry %d: got %q %v", entryIndex, content, err)
		}
	}
}

// Files dropped in the history folder by hand are not versions Docu-UI can vouch for.
func TestHistorySkipsForeignFiles(tester *testing.T) {
	folder, _ := editableFolder(tester, "A=1\n")
	entryFolder := filepath.Join(folder, historyFolder, "keycloak.env")
	if err := os.MkdirAll(entryFolder, 0o700); err != nil {
		tester.Fatal(err)
	}
	for _, fileName := range []string{"notes.txt", "20261399T000000.000000000Z_admin.env", "20260925T080000.000000000Z_admin.env"} {
		writeFile(tester, filepath.Join(entryFolder, fileName), "A=0\n")
	}
	entries, err := History(folder, "keycloak.env")
	if err != nil || len(entries) != 1 || entries[0].ID != "20260925T080000.000000000Z_admin.env" {
		tester.Fatalf("got %+v %v", entries, err)
	}
}

// A file never saved through Docu-UI has no history yet; that is not an error.
func TestHistoryOfUnsavedFileIsEmptyList(tester *testing.T) {
	folder, _ := editableFolder(tester, "A=1\n")
	entries, err := History(folder, "keycloak.env")
	if err != nil || entries == nil || len(entries) != 0 {
		tester.Fatalf("got %#v %v", entries, err)
	}
}

func TestHistoryErrors(tester *testing.T) {
	folder, _ := editableFolder(tester, "A=1\n")
	if _, err := History(folder, "../x.env"); !errors.Is(err, ErrInvalidName) {
		tester.Fatalf("bad name: %v", err)
	}
	if _, err := History(filepath.Join(folder, "missing"), "keycloak.env"); err == nil {
		tester.Fatal("missing folder must fail")
	}
	// A file where the history folder should be is a real problem, not "no history".
	if err := os.MkdirAll(filepath.Join(folder, historyFolder), 0o700); err != nil {
		tester.Fatal(err)
	}
	writeFile(tester, filepath.Join(folder, historyFolder, "keycloak.env"), "")
	if _, err := History(folder, "keycloak.env"); err == nil {
		tester.Fatal("unreadable history must fail")
	}
}

func TestHistoryContentErrors(tester *testing.T) {
	folder, _ := editableFolder(tester, "A=1\n")
	validID := "20260925T080000.000000000Z_admin.env"
	if _, err := HistoryContent(folder, "../x.env", validID); !errors.Is(err, ErrInvalidName) {
		tester.Fatalf("bad name: %v", err)
	}
	// The ID comes from the URL; anything that is not an entry name must not reach the disk.
	for _, badID := range []string{"../keycloak.env", "x/../" + validID, "keycloak.env", ""} {
		if _, err := HistoryContent(folder, "keycloak.env", badID); !errors.Is(err, ErrHistoryNotFound) {
			tester.Fatalf("%q: %v", badID, err)
		}
	}
	if _, err := HistoryContent(folder, "keycloak.env", validID); !errors.Is(err, ErrHistoryNotFound) {
		tester.Fatalf("missing entry: %v", err)
	}
	if _, err := HistoryContent(filepath.Join(folder, "missing"), "keycloak.env", validID); err == nil {
		tester.Fatal("missing folder must fail")
	}
	if err := os.MkdirAll(filepath.Join(folder, historyFolder, "keycloak.env", validID), 0o700); err != nil {
		tester.Fatal(err)
	}
	if _, err := HistoryContent(folder, "keycloak.env", validID); err == nil || errors.Is(err, ErrHistoryNotFound) {
		tester.Fatalf("unreadable entry: %v", err)
	}
}
