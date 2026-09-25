-- App settings changed from the web UI, so they apply without a restart.
CREATE TABLE settings (
    name  TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
