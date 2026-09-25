package store

import (
	"context"
	"database/sql"
	"errors"
)

const envFolderSetting = "env_folder"

// EnvFolder returns the folder that holds the env files, or "" when it was never set.
func (store *Store) EnvFolder(ctx context.Context) (string, error) {
	var folder string
	err := store.database.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = ?`, envFolderSetting).Scan(&folder)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return folder, err
}

// SetEnvFolder saves the env folder; the next request already uses it.
func (store *Store) SetEnvFolder(ctx context.Context, folder string) error {
	_, err := store.database.ExecContext(ctx, `
		INSERT INTO settings (name, value) VALUES (?, ?)
		ON CONFLICT (name) DO UPDATE SET value = excluded.value`, envFolderSetting, folder)
	return err
}
