package store

import (
	"context"
	"strings"
)

// ReplaceRecoveryCodes drops the account's codes and stores codeHashes instead. The old codes go
// first: if the insert then fails, the account has no codes rather than old ones still working.
func (store *Store) ReplaceRecoveryCodes(ctx context.Context, accountID int64, codeHashes []string) error {
	if _, err := store.database.ExecContext(ctx, `DELETE FROM recovery_codes WHERE account_id = ?`, accountID); err != nil {
		return err
	}
	placeholders := make([]string, len(codeHashes))
	arguments := make([]any, 0, 2*len(codeHashes))
	for index, codeHash := range codeHashes {
		placeholders[index] = "(?, ?)"
		arguments = append(arguments, accountID, codeHash)
	}
	_, err := store.database.ExecContext(ctx,
		`INSERT INTO recovery_codes (account_id, code_hash) VALUES `+strings.Join(placeholders, ", "), arguments...)
	return err
}

// UseRecoveryCode deletes the code and reports whether it existed. Check and delete are one
// statement, so two requests with the same code cannot both pass.
func (store *Store) UseRecoveryCode(ctx context.Context, accountID int64, codeHash string) (bool, error) {
	result, err := store.database.ExecContext(ctx,
		`DELETE FROM recovery_codes WHERE account_id = ? AND code_hash = ?`, accountID, codeHash)
	if err != nil {
		return false, err
	}
	deletedRows, _ := result.RowsAffected() // SQLite always reports affected rows
	return deletedRows == 1, nil
}

// CountRecoveryCodes returns how many unused codes the account has left.
func (store *Store) CountRecoveryCodes(ctx context.Context, accountID int64) (int, error) {
	var codeCount int
	err := store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM recovery_codes WHERE account_id = ?`, accountID).Scan(&codeCount)
	return codeCount, err
}
