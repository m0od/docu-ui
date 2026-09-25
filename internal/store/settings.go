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

	webhookURLSetting         = "webhook_url"
	webhookSecretSetting      = "webhook_secret"
	webhookHeaderNameSetting  = "webhook_header_name"
	webhookHeaderValueSetting = "webhook_header_value"
)

// DocoCD is how Docu-UI reaches the Doco-CD REST API.
type DocoCD struct {
	URL    string
	APIKey string
}

// Webhook is where Apply posts, for a CI/CD system to recreate the services.
type Webhook struct {
	URL string
	// Signs the body with HMAC-SHA256; empty sends no signature.
	Secret string
	// An extra header such as "Authorization: Bearer ...", for receivers that check a token.
	HeaderName  string
	HeaderValue string
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

// Webhook returns the shared webhook; empty fields were never set.
func (store *Store) Webhook(ctx context.Context) (Webhook, error) {
	var webhook Webhook
	for _, field := range []struct {
		name  string
		value *string
	}{
		{webhookURLSetting, &webhook.URL},
		{webhookSecretSetting, &webhook.Secret},
		{webhookHeaderNameSetting, &webhook.HeaderName},
		{webhookHeaderValueSetting, &webhook.HeaderValue},
	} {
		value, err := store.setting(ctx, field.name)
		if err != nil {
			return Webhook{}, err
		}
		*field.value = value
	}
	return webhook, nil
}

// SetWebhook saves every field in one statement, so a URL is never paired with another receiver's secret.
func (store *Store) SetWebhook(ctx context.Context, webhook Webhook) error {
	_, err := store.database.ExecContext(ctx, `
		INSERT INTO settings (name, value) VALUES (?, ?), (?, ?), (?, ?), (?, ?)
		ON CONFLICT (name) DO UPDATE SET value = excluded.value`,
		webhookURLSetting, webhook.URL, webhookSecretSetting, webhook.Secret,
		webhookHeaderNameSetting, webhook.HeaderName, webhookHeaderValueSetting, webhook.HeaderValue)
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
