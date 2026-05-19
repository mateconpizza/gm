-- migration: 0003_indexes
-- description: add indexes to optimize queries on frequent lookups, filtering,
-- and sorting fields.

-- speed up duplicate checks and URL lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_bookmarks_url
ON bookmarks(url);

-- optimize filtering by active status and response codes
CREATE INDEX IF NOT EXISTS idx_bookmarks_status
ON bookmarks(is_active, status_code);

-- improve cron/worker queries for dead-link checking
CREATE INDEX IF NOT EXISTS idx_bookmarks_last_checked
ON bookmarks(last_checked);

-- speed up dashboard filtering for favorited items
CREATE INDEX IF NOT EXISTS idx_bookmarks_favorite
ON bookmarks(favorite);

-- optimize chronological sorting (e.g., recent bookmarks)
CREATE INDEX IF NOT EXISTS idx_bookmarks_created_at
ON bookmarks(created_at);

-- speed up "most visited" analytics views
CREATE INDEX IF NOT EXISTS idx_bookmarks_visit_count
ON bookmarks(visit_count DESC);

-- enforce unique tag names and speed up tag search
CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_name
ON tags(name);

-- optimize join table performance for many-to-many relations
CREATE INDEX IF NOT EXISTS idx_bookmark_tags
ON bookmark_tags(bookmark_id, tag_id);
