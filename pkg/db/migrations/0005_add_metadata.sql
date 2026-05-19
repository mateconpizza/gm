-- migration: 0005_add_metadata
-- description: add metadata table to store application state, schema
-- telemetry, and system versions.

-- key-value store for app configuration and environment details
CREATE TABLE IF NOT EXISTS metadata (
    key     TEXT PRIMARY KEY,
    value   TEXT NOT NULL
);

-- seed initial environmental metadata
INSERT OR IGNORE INTO metadata (key, value)
VALUES
    ('created_at', CURRENT_TIMESTAMP),
    ('app_version', 'dev'),
    ('sqlite_version', sqlite_version());
