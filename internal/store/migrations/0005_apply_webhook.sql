-- How a file is applied: 'doco-cd' (existing rows) or 'webhook'.
ALTER TABLE apply_targets ADD COLUMN adapter TEXT NOT NULL DEFAULT 'doco-cd';
-- A file's own webhook; an empty URL means it uses the shared one from settings.
ALTER TABLE apply_targets ADD COLUMN webhook_url TEXT NOT NULL DEFAULT '';
ALTER TABLE apply_targets ADD COLUMN webhook_secret TEXT NOT NULL DEFAULT '';
ALTER TABLE apply_targets ADD COLUMN webhook_header_name TEXT NOT NULL DEFAULT '';
ALTER TABLE apply_targets ADD COLUMN webhook_header_value TEXT NOT NULL DEFAULT '';
