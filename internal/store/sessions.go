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
		`INSERT INTO sessions (token_hash, account_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, accountID, expiresAt.Unix())
	return err
}

// FindSessionUsername returns the username of a live session.
func (store *Store) FindSessionUsername(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	var username string
	err := store.database.QueryRowContext(ctx, `
		SELECT accounts.username FROM sessions JOIN accounts ON accounts.id = sessions.account_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ?`,
		tokenHash, now.Unix()).Scan(&username)
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
