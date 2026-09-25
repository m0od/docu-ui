package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Each file is one schema version: 0001_*.sql is version 1, 0002_*.sql is version 2, and so on.
// Never edit a released file; add the next number instead, because existing databases already ran it.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrate applies every migration newer than the database's PRAGMA user_version.
// Each migration and its version bump commit together, so a crash never leaves a half-applied version.
func migrate(database *sql.DB, migrations fs.FS) error {
	fileNames, _ := fs.Glob(migrations, "migrations/*.sql") // the pattern is a constant, so Glob cannot fail
	sort.Strings(fileNames)

	var currentVersion int
	if err := database.QueryRow(`PRAGMA user_version`).Scan(&currentVersion); err != nil {
		return err
	}
	if currentVersion > len(fileNames) {
		return fmt.Errorf("database schema is version %d but this build only knows %d: run a newer Docu-UI", currentVersion, len(fileNames))
	}
	for fileIndex, fileName := range fileNames {
		version := fileIndex + 1
		if expectedPrefix := fmt.Sprintf("migrations/%04d_", version); !strings.HasPrefix(fileName, expectedPrefix) {
			return fmt.Errorf("migration %s: expected name starting with %s (numbers must have no gaps)", fileName, expectedPrefix)
		}
		if version <= currentVersion {
			continue
		}
		statements, _ := fs.ReadFile(migrations, fileName) // listed by Glob just above
		if err := applyMigration(database, version, string(statements)); err != nil {
			return fmt.Errorf("migration %s: %w", fileName, err)
		}
	}
	return nil
}

func applyMigration(database *sql.DB, version int, statements string) error {
	transaction, err := database.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback() // no-op after Commit
	// One Exec for both, so the version bump can only succeed together with the schema change.
	if _, err := transaction.Exec(statements + fmt.Sprintf("\n;PRAGMA user_version = %d;", version)); err != nil {
		return err
	}
	return transaction.Commit()
}
