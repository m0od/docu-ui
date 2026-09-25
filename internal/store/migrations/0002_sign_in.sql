-- Times are unix seconds.
ALTER TABLE accounts ADD COLUMN failed_logins INTEGER NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN locked_until INTEGER NOT NULL DEFAULT 0;
-- Last TOTP time step accepted, so a code cannot be used twice (RFC 6238 section 5.2).
ALTER TABLE accounts ADD COLUMN totp_last_step INTEGER NOT NULL DEFAULT 0;

CREATE TABLE sessions (
	token_hash TEXT PRIMARY KEY, -- SHA-256 of the cookie value; a leaked database gives no usable sessions
	account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
