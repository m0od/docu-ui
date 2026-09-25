-- Where a saved env file is applied, and the file version applied last.
CREATE TABLE apply_targets (
    file_name       TEXT PRIMARY KEY,
    project         TEXT NOT NULL,
    services        TEXT NOT NULL, -- space-separated; empty means the whole project
    applied_version TEXT NOT NULL
);
