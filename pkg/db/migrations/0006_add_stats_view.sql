-- migration: 0006_add_stats_view
-- description: create a unified view to aggregate global database metrics and
-- application statistics.

-- virtual table for quick dashboard reporting and telemetry
CREATE VIEW IF NOT EXISTS stats AS
SELECT
    -- total repository scale
    (SELECT COUNT(*) FROM bookmarks)
        AS total_bookmarks,

    (SELECT COUNT(*) FROM tags)
        AS total_tags,

    -- filtered bookmark states
    (SELECT COUNT(*) FROM bookmarks WHERE favorite = 1)
        AS favorites,

    (SELECT COUNT(*) FROM bookmarks WHERE archive_url != '')
        AS archived,

    (SELECT COUNT(*) FROM bookmarks WHERE is_active = 0)
        AS dead_links,

    -- user engagement metrics
    (SELECT COALESCE(SUM(visit_count), 0) FROM bookmarks)
        AS total_visits;
