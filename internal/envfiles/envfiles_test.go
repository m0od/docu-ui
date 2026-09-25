package envfiles

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(tester *testing.T, path, content string) {
	tester.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		tester.Fatal(err)
	}
}

func TestCheckFolder(tester *testing.T) {
	folder := tester.TempDir()
	if err := CheckFolder(folder); err != nil {
		tester.Fatalf("existing folder: %v", err)
	}
	// A relative path would depend on the container's working directory.
	if err := CheckFolder("env"); err == nil || !strings.Contains(err.Error(), "absolute") {
		tester.Fatalf("relative path: %v", err)
	}
	// Typo or folder not mounted into the container: refuse before saving.
	if err := CheckFolder(filepath.Join(folder, "missing")); err == nil {
		tester.Fatal("missing folder accepted")
	}
	filePath := filepath.Join(folder, "a.env")
	writeFile(tester, filePath, "")
	if err := CheckFolder(filePath); err == nil {
		tester.Fatal("a file accepted as folder")
	}
}

func TestListShowsOnlyEnvFilesInTheFolder(tester *testing.T) {
	folder := tester.TempDir()
	for _, fileName := range []string{"keycloak.env", "airflow.env", "notes.txt", ".hidden.env", "env"} {
		writeFile(tester, filepath.Join(folder, fileName), "")
	}
	os.Mkdir(filepath.Join(folder, "nested.env"), 0o700)
	os.Symlink("/etc/passwd", filepath.Join(folder, "link.env"))

	fileNames, err := List(folder)
	if err != nil {
		tester.Fatal(err)
	}
	if want := []string{"airflow.env", "keycloak.env"}; !reflect.DeepEqual(fileNames, want) {
		tester.Fatalf("got %v want %v", fileNames, want)
	}
}

// The UI renders "no files", not null.
func TestListEmptyFolderIsEmptyList(tester *testing.T) {
	fileNames, err := List(tester.TempDir())
	if err != nil || fileNames == nil || len(fileNames) != 0 {
		tester.Fatalf("got %#v, %v", fileNames, err)
	}
}

// The folder can disappear after it was saved (volume unmounted).
func TestListMissingFolder(tester *testing.T) {
	if _, err := List(filepath.Join(tester.TempDir(), "gone")); err == nil {
		tester.Fatal("expected error")
	}
}

const sampleEnv = `# Keycloak
KC_DB=postgres
export KC_HOSTNAME = https://id.example.com
KC_DB_PASSWORD="p@ss=word"

this line is broken
1BAD=x
KC_DB=mariadb
`

func TestReadListsKeysWithoutValues(tester *testing.T) {
	folder := tester.TempDir()
	windowsContent := strings.ReplaceAll(sampleEnv, "\n", "\r\n")
	writeFile(tester, filepath.Join(folder, "keycloak.env"), windowsContent)

	envFile, err := Read(folder, "keycloak.env")
	if err != nil {
		tester.Fatal(err)
	}
	want := File{
		Name:    "keycloak.env",
		Version: contentVersion(windowsContent),
		Variables: []Variable{
			{Key: "KC_DB", Line: 2, Overridden: true},
			{Key: "KC_HOSTNAME", Line: 3},
			{Key: "KC_DB_PASSWORD", Line: 4},
			{Key: "KC_DB", Line: 8},
		},
		InvalidLines: []int{6, 7},
	}
	if !reflect.DeepEqual(envFile, want) {
		tester.Fatalf("got %+v\nwant %+v", envFile, want)
	}
}

func TestValue(tester *testing.T) {
	folder := tester.TempDir()
	writeFile(tester, filepath.Join(folder, "keycloak.env"), sampleEnv)

	expectedValues := map[string]string{
		// Compose uses the last line when a key repeats.
		"KC_DB":       "mariadb",
		"KC_HOSTNAME": "https://id.example.com",
		// Shown as written, so the editor later saves exactly what the user sees.
		"KC_DB_PASSWORD": `"p@ss=word"`,
	}
	for key, expectedValue := range expectedValues {
		if value, err := Value(folder, "keycloak.env", key); err != nil || value != expectedValue {
			tester.Errorf("%s: got %q, %v", key, value, err)
		}
	}
	if _, err := Value(folder, "keycloak.env", "MISSING"); !errors.Is(err, ErrVariableNotFound) {
		tester.Errorf("missing key: %v", err)
	}
}

// File names come from the URL; none of these may read outside the folder.
func TestReadRejectsNamesOutsideTheFolder(tester *testing.T) {
	folder := tester.TempDir()
	for _, fileName := range []string{"../secret.env", "/etc/passwd", "sub/a.env", ".env", "a.txt", ""} {
		if _, err := Read(folder, fileName); !errors.Is(err, ErrInvalidName) {
			tester.Errorf("%q: %v", fileName, err)
		}
		if _, err := Value(folder, fileName, "KEY"); !errors.Is(err, ErrInvalidName) {
			tester.Errorf("value %q: %v", fileName, err)
		}
	}
}

func TestReadMissingFile(tester *testing.T) {
	if _, err := Read(tester.TempDir(), "missing.env"); !errors.Is(err, ErrFileNotFound) {
		tester.Fatal(err)
	}
}

func TestReadMissingFolder(tester *testing.T) {
	_, err := Read(filepath.Join(tester.TempDir(), "gone"), "a.env")
	if err == nil || errors.Is(err, ErrFileNotFound) {
		tester.Fatalf("got %v", err)
	}
}

// A symlink placed in the folder must not expose files elsewhere on the host.
func TestReadRefusesSymlinkOutOfTheFolder(tester *testing.T) {
	outside := tester.TempDir()
	writeFile(tester, filepath.Join(outside, "secret.env"), "TOKEN=abc")
	folder := tester.TempDir()
	os.Symlink(filepath.Join(outside, "secret.env"), filepath.Join(folder, "link.env"))

	_, err := Read(folder, "link.env")
	if err == nil || errors.Is(err, ErrFileNotFound) {
		tester.Fatalf("got %v", err)
	}
}
