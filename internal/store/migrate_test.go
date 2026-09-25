package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func openRawDatabase(tester *testing.T) *sql.DB {
	tester.Helper()
	database, _ := sql.Open("sqlite", "file:"+filepath.Join(tester.TempDir(), "raw.db"))
	tester.Cleanup(func() { database.Close() })
	return database
}

func schemaVersion(tester *testing.T, database *sql.DB) int {
	tester.Helper()
	var version int
	if err := database.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		tester.Fatal(err)
	}
	return version
}

// Only migrations newer than the stored version run, so upgrading keeps existing accounts.
func TestMigrateAppliesOnlyNewVersions(tester *testing.T) {
	database := openRawDatabase(tester)
	firstRelease := fstest.MapFS{"migrations/0001_items.sql": {Data: []byte(`CREATE TABLE items (name TEXT);`)}}
	if err := migrate(database, firstRelease); err != nil {
		tester.Fatal(err)
	}
	database.Exec(`INSERT INTO items VALUES ('kept')`)

	secondRelease := fstest.MapFS{
		"migrations/0001_items.sql":      firstRelease["migrations/0001_items.sql"],
		"migrations/0002_item_color.sql": {Data: []byte(`ALTER TABLE items ADD COLUMN color TEXT;`)},
	}
	if err := migrate(database, secondRelease); err != nil {
		tester.Fatalf("upgrade re-ran version 1 or failed: %v", err)
	}
	var keptName string
	database.QueryRow(`SELECT name FROM items`).Scan(&keptName)
	if schemaVersion(tester, database) != 2 || keptName != "kept" {
		tester.Fatalf("version %d, row %q", schemaVersion(tester, database), keptName)
	}
}

// A broken migration must leave the database at the old version, so the next start retries it cleanly.
func TestFailedMigrationKeepsOldVersion(tester *testing.T) {
	database := openRawDatabase(tester)
	brokenRelease := fstest.MapFS{
		"migrations/0001_items.sql":  {Data: []byte(`CREATE TABLE items (name TEXT);`)},
		"migrations/0002_broken.sql": {Data: []byte(`CREATE TABLE half (id INTEGER); THIS IS NOT SQL;`)},
	}
	if err := migrate(database, brokenRelease); err == nil || !strings.Contains(err.Error(), "0002_broken.sql") {
		tester.Fatalf("got %v", err)
	}
	var halfTableCount int
	database.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'half'`).Scan(&halfTableCount)
	if schemaVersion(tester, database) != 1 || halfTableCount != 0 {
		tester.Fatalf("version %d, half-applied table count %d", schemaVersion(tester, database), halfTableCount)
	}
}

// Rolling back to an older image must stop, not run against a schema it does not understand.
func TestMigrateRefusesNewerDatabase(tester *testing.T) {
	database := openRawDatabase(tester)
	database.Exec(`PRAGMA user_version = 5`)
	if err := migrate(database, migrationFiles); err == nil || !strings.Contains(err.Error(), "newer Docu-UI") {
		tester.Fatalf("got %v", err)
	}
}

// A gap (0001, 0003) would silently shift every later version number.
func TestMigrateRejectsNumberingGap(tester *testing.T) {
	gappedRelease := fstest.MapFS{
		"migrations/0001_items.sql": {Data: []byte(`CREATE TABLE items (name TEXT);`)},
		"migrations/0003_skip.sql":  {Data: []byte(`SELECT 1;`)},
	}
	if err := migrate(openRawDatabase(tester), gappedRelease); err == nil || !strings.Contains(err.Error(), "no gaps") {
		tester.Fatalf("got %v", err)
	}
}

// The real migration files shipped in the binary must be well-numbered and apply cleanly.
func TestShippedMigrationsApply(tester *testing.T) {
	database := openRawDatabase(tester)
	if err := migrate(database, migrationFiles); err != nil {
		tester.Fatal(err)
	}
	if schemaVersion(tester, database) < 1 {
		tester.Fatal("no shipped migration ran")
	}
}

func TestMigrateFailsOnClosedDatabase(tester *testing.T) {
	database := openRawDatabase(tester)
	database.Close()
	if err := migrate(database, migrationFiles); err == nil {
		tester.Error("migrate on closed database should fail")
	}
	if err := applyMigration(database, 1, `SELECT 1;`); err == nil {
		tester.Error("applyMigration on closed database should fail")
	}
}
