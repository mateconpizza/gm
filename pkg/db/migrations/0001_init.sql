-- migration: 0001_init
-- description: initial database schema setup.
--
-- Note: `bookmark_tags` uses `bookmark_url` here to preserve `legacy` state.
-- it will be refactored to use `bookmark_id` in migration 0002.

pragma foreign_keys = on
;

-- core tables
CREATE TABLE IF NOT EXISTS bookmarks (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    url                 TEXT NOT NULL UNIQUE,
    title               TEXT DEFAULT "",
    desc                TEXT DEFAULT "",
    notes               TEXT DEFAULT "",
    created_at          TEXT DEFAULT CURRENT_TIMESTAMP,
    last_visit          TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at          TEXT DEFAULT CURRENT_TIMESTAMP,
    visit_count         INTEGER DEFAULT 0 CHECK(visit_count >= 0),
    favorite            BOOLEAN DEFAULT FALSE,
    favicon_url         TEXT DEFAULT "",
    favicon_local       TEXT DEFAULT "",
    archive_url         TEXT DEFAULT "",
    archive_timestamp   TEXT DEFAULT "",
    checksum            TEXT NOT NULL,
    last_checked        TEXT DEFAULT "",
    status_code         INTEGER DEFAULT 0 CHECK(status_code >= 0),
    status_text         TEXT DEFAULT "",
    is_active           BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS tags (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    name    TEXT NOT NULL UNIQUE
);

-- join table (legacy version tracking bookmarks by url)
CREATE TABLE IF NOT EXISTS bookmark_tags (
    bookmark_url   TEXT NOT NULL,
    tag_id         INTEGER NOT NULL,

    FOREIGN KEY (tag_id)
        REFERENCES tags(id)
        ON DELETE CASCADE,

    PRIMARY KEY (bookmark_url, tag_id)
);
