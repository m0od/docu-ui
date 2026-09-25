package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ErrNoApplyTarget means the env file was never set up to be applied.
var ErrNoApplyTarget = errors.New("no apply target")

// ApplyTarget says which Compose project and services use an env file.
type ApplyTarget struct {
	Project string
	// Empty means the whole project.
	Services []string
	// The file version applied last; a different current version means saved but not applied.
	AppliedVersion string
}

// ApplyTarget returns the target of fileName, or ErrNoApplyTarget.
func (store *Store) ApplyTarget(ctx context.Context, fileName string) (ApplyTarget, error) {
	var target ApplyTarget
	var services string
	err := store.database.QueryRowContext(ctx,
		`SELECT project, services, applied_version FROM apply_targets WHERE file_name = ?`, fileName,
	).Scan(&target.Project, &services, &target.AppliedVersion)
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
		INSERT INTO apply_targets (file_name, project, services, applied_version) VALUES (?, ?, ?, ?)
		ON CONFLICT (file_name) DO UPDATE SET
			project = excluded.project, services = excluded.services, applied_version = excluded.applied_version`,
		fileName, target.Project, strings.Join(target.Services, " "), target.AppliedVersion)
	return err
}
