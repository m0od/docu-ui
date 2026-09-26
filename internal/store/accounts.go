package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrAccountNotFound means no account has the given username.
var ErrAccountNotFound = errors.New("account not found")

// Account is a sign-in identity.
type Account struct {
	ID           int64
	Username     string
	PasswordHash string
	TOTPSecret   string // empty = TOTP not enabled
	LockedUntil  time.Time
}

// FindAccount returns the account with username.
func (store *Store) FindAccount(ctx context.Context, username string) (Account, error) {
	var account Account
	var lockedUntilUnix int64
	err := store.database.QueryRowContext(ctx, `
		SELECT id, username, password_hash, totp_secret, locked_until FROM accounts WHERE username = ?`,
		username).Scan(&account.ID, &account.Username, &account.PasswordHash, &account.TOTPSecret, &lockedUntilUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	account.LockedUntil = time.Unix(lockedUntilUnix, 0)
	return account, err
}

// RecordFailedLogin counts one failed sign-in. The failure that reaches maxFailures locks the
// account until lockUntil and starts the count again. One UPDATE, so parallel guesses all count.
func (store *Store) RecordFailedLogin(ctx context.Context, accountID int64, maxFailures int, lockUntil time.Time) error {
	_, err := store.database.ExecContext(ctx, `
		UPDATE accounts SET
			locked_until  = CASE WHEN failed_logins + 1 >= ? THEN ? ELSE locked_until END,
			failed_logins = CASE WHEN failed_logins + 1 >= ? THEN 0 ELSE failed_logins + 1 END
		WHERE id = ?`,
		maxFailures, lockUntil.Unix(), maxFailures, accountID)
	return err
}

// ResetFailedLogins clears the failure count after a successful sign-in.
func (store *Store) ResetFailedLogins(ctx context.Context, accountID int64) error {
	_, err := store.database.ExecContext(ctx, `UPDATE accounts SET failed_logins = 0 WHERE id = ?`, accountID)
	return err
}

// UseTOTPStep records timeStep as used and reports false if it is not newer than the last used step.
// The check and the write are one statement, so two requests with the same code cannot both pass.
func (store *Store) UseTOTPStep(ctx context.Context, accountID, timeStep int64) (bool, error) {
	result, err := store.database.ExecContext(ctx, `
		UPDATE accounts SET totp_last_step = ? WHERE id = ? AND totp_last_step < ?`,
		timeStep, accountID, timeStep)
	if err != nil {
		return false, err
	}
	updatedRows, _ := result.RowsAffected() // SQLite always reports affected rows
	return updatedRows == 1, nil
}

// SetPassword replaces the password hash of accountID.
func (store *Store) SetPassword(ctx context.Context, accountID int64, passwordHash string) error {
	_, err := store.database.ExecContext(ctx, `UPDATE accounts SET password_hash = ? WHERE id = ?`, passwordHash, accountID)
	return err
}

// SetTOTP turns TOTP on with secret, or off with "". lastStep is the time step of the code that
// proved the app works, so that same code cannot sign in again.
func (store *Store) SetTOTP(ctx context.Context, accountID int64, secret string, lastStep int64) error {
	_, err := store.database.ExecContext(ctx,
		`UPDATE accounts SET totp_secret = ?, totp_last_step = ? WHERE id = ?`, secret, lastStep, accountID)
	return err
}
