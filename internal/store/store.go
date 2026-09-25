// Package store keeps Docu-UI's own state (accounts) in a single SQLite file.
package store

import (
	"context"
	"database/sql"
	"errors"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, so the static distroless image still works
)

// ErrSetupDone means an account already exists, so first-run setup is closed.
var ErrSetupDone = errors.New("setup already completed")

// Store is the database handle.
type Store struct {
	database *sql.DB
}

// Open opens (creating if needed) the SQLite file at databasePath and brings its schema up to date.
func Open(databasePath string) (*Store, error) {
	// sql.Open only fails for an unregistered driver name; the import above registers "sqlite".
	database, _ := sql.Open("sqlite", "file:"+databasePath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err := migrate(database, migrationFiles); err != nil {
		database.Close()
		return nil, err
	}
	return &Store{database: database}, nil
}

// Close releases the database file.
func (store *Store) Close() error {
	return store.database.Close()
}

// NeedsSetup reports whether no account exists yet.
func (store *Store) NeedsSetup(ctx context.Context) (bool, error) {
	var accountExists bool
	err := store.database.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM accounts)`).Scan(&accountExists)
	return !accountExists, err
}

// CreateFirstAccount inserts the admin account only while the table is empty.
// Check and insert are one statement, so two setup requests racing cannot both succeed.
func (store *Store) CreateFirstAccount(ctx context.Context, username, passwordHash, totpSecret string) error {
	result, err := store.database.ExecContext(ctx, `
		INSERT INTO accounts (username, password_hash, totp_secret)
		SELECT ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM accounts)`,
		username, passwordHash, totpSecret)
	if err != nil {
		return err
	}
	insertedRows, _ := result.RowsAffected() // SQLite always reports affected rows
	if insertedRows == 0 {
		return ErrSetupDone
	}
	return nil
}
