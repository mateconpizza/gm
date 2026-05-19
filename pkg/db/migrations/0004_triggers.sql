-- migration: 0004_triggers
-- description: add database triggers to automate metadata updates.

-- automatically refresh the `updated_at` timestamp whenever any core bookmark
-- field changes
CREATE TRIGGER IF NOT EXISTS update_bookmark_updated_at
AFTER UPDATE OF
    url,
    title,
    desc,
    notes,
    visit_count,
    favorite,
    favicon_url,
    favicon_local,
    archive_url,
    archive_timestamp,
    checksum,
    last_checked,
    status_code,
    status_text,
    is_active
ON bookmarks
FOR EACH ROW
BEGIN
    UPDATE bookmarks
    SET updated_at = CURRENT_TIMESTAMP
    WHERE id = OLD.id;
END;
