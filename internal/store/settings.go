package store

import (
	"context"
	"database/sql"
	"errors"
)

const (
	envFolderSetting    = "env_folder"
	docoCDURLSetting    = "doco_cd_url"
	docoCDAPIKeySetting = "doco_cd_api_key"
)

// DocoCD is how Docu-UI reaches the Doco-CD REST API.
type DocoCD struct {
	URL    string
	APIKey string
}

// EnvFolder returns the folder that holds the env files, or "" when it was never set.
func (store *Store) EnvFolder(ctx context.Context) (string, error) {
	return store.setting(ctx, envFolderSetting)
}

// SetEnvFolder saves the env folder; the next request already uses it.
func (store *Store) SetEnvFolder(ctx context.Context, folder string) error {
	_, err := store.database.ExecContext(ctx, `
		INSERT INTO settings (name, value) VALUES (?, ?)
		ON CONFLICT (name) DO UPDATE SET value = excluded.value`, envFolderSetting, folder)
	return err
}

// DocoCD returns the Doco-CD settings; empty fields were never set.
func (store *Store) DocoCD(ctx context.Context) (DocoCD, error) {
	url, err := store.setting(ctx, docoCDURLSetting)
	if err != nil {
		return DocoCD{}, err
	}
	apiKey, err := store.setting(ctx, docoCDAPIKeySetting)
	return DocoCD{URL: url, APIKey: apiKey}, err
}

// SetDocoCD saves both fields in one statement, so a URL is never paired with another server's key.
func (store *Store) SetDocoCD(ctx context.Context, settings DocoCD) error {
	_, err := store.database.ExecContext(ctx, `
		INSERT INTO settings (name, value) VALUES (?, ?), (?, ?)
		ON CONFLICT (name) DO UPDATE SET value = excluded.value`,
		docoCDURLSetting, settings.URL, docoCDAPIKeySetting, settings.APIKey)
	return err
}

func (store *Store) setting(ctx context.Context, name string) (string, error) {
	var value string
	err := store.database.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = ?`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}
