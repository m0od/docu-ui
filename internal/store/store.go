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
	database, _ := sql.Open("sqlite", "file:"+databasePath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
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

// signInSetting is "off" when the admin chose to run without sign-in. It only counts while no
// account exists: an account always means sign-in, so a leftover "off" can never open the UI.
const signInSetting = "sign_in"

// NeedsSetup reports whether no account exists yet and sign-in was not turned off.
func (store *Store) NeedsSetup(ctx context.Context) (bool, error) {
	var needsSetup bool
	err := store.database.QueryRowContext(ctx, `
		SELECT NOT EXISTS (SELECT 1 FROM accounts)
		   AND NOT EXISTS (SELECT 1 FROM settings WHERE name = ? AND value = 'off')`, signInSetting).Scan(&needsSetup)
	return needsSetup, err
}

// SignInOff reports whether Docu-UI runs without sign-in: no account, and sign-in turned off.
func (store *Store) SignInOff(ctx context.Context) (bool, error) {
	var signInOff bool
	err := store.database.QueryRowContext(ctx, `
		SELECT NOT EXISTS (SELECT 1 FROM accounts)
		   AND EXISTS (SELECT 1 FROM settings WHERE name = ? AND value = 'off')`, signInSetting).Scan(&signInOff)
	return signInOff, err
}

// SkipSignIn turns sign-in off at first setup. Like CreateFirstAccount it only works while no
// account exists, in one statement, so it cannot race with a setup that creates one.
func (store *Store) SkipSignIn(ctx context.Context) error {
	result, err := store.database.ExecContext(ctx, `
		INSERT INTO settings (name, value) SELECT ?, 'off' WHERE NOT EXISTS (SELECT 1 FROM accounts)
		ON CONFLICT (name) DO UPDATE SET value = excluded.value`, signInSetting)
	if err != nil {
		return err
	}
	changedRows, _ := result.RowsAffected() // SQLite always reports affected rows
	if changedRows == 0 {
		return ErrSetupDone
	}
	return nil
}

// TurnOffSignIn deletes the accounts (and by cascade their sessions) and turns sign-in off.
// The setting is written first: if deleting the accounts then fails, an account still exists
// and sign-in stays on.
func (store *Store) TurnOffSignIn(ctx context.Context) error {
	if _, err := store.database.ExecContext(ctx, `
		INSERT INTO settings (name, value) VALUES (?, 'off')
		ON CONFLICT (name) DO UPDATE SET value = excluded.value`, signInSetting); err != nil {
		return err
	}
	_, err := store.database.ExecContext(ctx, `DELETE FROM accounts`)
	return err
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
