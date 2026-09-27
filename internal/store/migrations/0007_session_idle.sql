-- Last request made with the session, for the idle timeout (unix seconds). Sessions from before
-- this migration start at 0, so they count as idle and their users sign in once more.
ALTER TABLE sessions ADD COLUMN last_used_at INTEGER NOT NULL DEFAULT 0;
