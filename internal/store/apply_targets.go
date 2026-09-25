package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ErrNoApplyTarget means the env file was never set up to be applied.
var ErrNoApplyTarget = errors.New("no apply target")

// Adapters an env file can be applied with.
const (
	AdapterDocoCD  = "doco-cd"
	AdapterWebhook = "webhook"
)

// ApplyTarget says how an env file is applied and which Compose project and services use it.
type ApplyTarget struct {
	Adapter string
	Project string
	// Empty means the whole project.
	Services []string
	// The file version applied last; a different current version means saved but not applied.
	AppliedVersion string
	// The file's own webhook; an empty URL means the shared one from settings.
	Webhook Webhook
}

// ApplyTarget returns the target of fileName, or ErrNoApplyTarget.
func (store *Store) ApplyTarget(ctx context.Context, fileName string) (ApplyTarget, error) {
	var target ApplyTarget
	var services string
	err := store.database.QueryRowContext(ctx,
		`SELECT adapter, project, services, applied_version,
			webhook_url, webhook_secret, webhook_header_name, webhook_header_value
		FROM apply_targets WHERE file_name = ?`, fileName,
	).Scan(&target.Adapter, &target.Project, &services, &target.AppliedVersion,
		&target.Webhook.URL, &target.Webhook.Secret, &target.Webhook.HeaderName, &target.Webhook.HeaderValue)
	if errors.Is(err, sql.ErrNoRows) {
		return ApplyTarget{}, ErrNoApplyTarget
	}
	// strings.Fields gives an empty (not nil) slice for "", so JSON shows [] rather than null.
	target.Services = append([]string{}, strings.Fields(services)...)
	return target, err
}

// SetApplyTarget creates or replaces the target of fileName.
func (store *Store) SetApplyTarget(ctx context.Context, fileName string, target ApplyTarget) error {
	_, err := store.database.ExecContext(ctx, `
		INSERT INTO apply_targets (file_name, adapter, project, services, applied_version,
			webhook_url, webhook_secret, webhook_header_name, webhook_header_value)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (file_name) DO UPDATE SET
			adapter = excluded.adapter, project = excluded.project, services = excluded.services,
			applied_version = excluded.applied_version, webhook_url = excluded.webhook_url,
			webhook_secret = excluded.webhook_secret, webhook_header_name = excluded.webhook_header_name,
			webhook_header_value = excluded.webhook_header_value`,
		fileName, target.Adapter, target.Project, strings.Join(target.Services, " "), target.AppliedVersion,
		target.Webhook.URL, target.Webhook.Secret, target.Webhook.HeaderName, target.Webhook.HeaderValue)
	return err
}
