package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrSessionNotFound means the session does not exist or has expired.
var ErrSessionNotFound = errors.New("session not found")

// CreateSession stores a session for accountID and drops expired ones, so the table does not grow forever.
func (store *Store) CreateSession(ctx context.Context, tokenHash string, accountID int64, now, expiresAt time.Time) error {
	if _, err := store.database.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		return err
	}
	_, err := store.database.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, account_id, expires_at, last_used_at) VALUES (?, ?, ?, ?)`,
		tokenHash, accountID, expiresAt.Unix(), now.Unix())
	return err
}

// UseSession returns the username of a session that has neither expired nor gone unused for
// idleTimeout, and records now as its last use: an active session lives until it expires.
func (store *Store) UseSession(ctx context.Context, tokenHash string, now time.Time, idleTimeout time.Duration) (string, error) {
	var username string
	err := store.database.QueryRowContext(ctx, `
		UPDATE sessions SET last_used_at = ?
		WHERE token_hash = ? AND expires_at > ? AND last_used_at > ?
		RETURNING (SELECT username FROM accounts WHERE accounts.id = sessions.account_id)`,
		now.Unix(), tokenHash, now.Unix(), now.Add(-idleTimeout).Unix()).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrSessionNotFound
	}
	return username, err
}

// DeleteSession ends a session (sign-out). Deleting an unknown session is not an error.
func (store *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := store.database.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// DeleteOtherSessions ends every session of accountID except keepTokenHash, e.g. after a password change.
func (store *Store) DeleteOtherSessions(ctx context.Context, accountID int64, keepTokenHash string) error {
	_, err := store.database.ExecContext(ctx,
		`DELETE FROM sessions WHERE account_id = ? AND token_hash != ?`, accountID, keepTokenHash)
	return err
}
