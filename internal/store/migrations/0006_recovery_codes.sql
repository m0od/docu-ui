-- One-time codes for signing in without the TOTP app. Only SHA-256 hashes are kept; a used code is deleted.
CREATE TABLE recovery_codes (
	account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	code_hash TEXT NOT NULL,
	PRIMARY KEY (account_id, code_hash)
);
