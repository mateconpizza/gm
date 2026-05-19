-- migration: 0002_bookmark_tags_refactor
-- description: replace `bookmark_url` with `bookmark_id` in relation table

-- rename current table to back up existing data
ALTER TABLE bookmark_tags RENAME TO bookmark_tags_old;

-- create the normalized table using bookmark_id
CREATE TABLE bookmark_tags (
    bookmark_id INTEGER NOT NULL,
    tag_id      INTEGER NOT NULL,

    FOREIGN KEY (bookmark_id)
        REFERENCES bookmarks(id)
        ON DELETE CASCADE,

    FOREIGN KEY (tag_id)
        REFERENCES tags(id)
        ON DELETE CASCADE,

    PRIMARY KEY (bookmark_id, tag_id)
);

-- map old URLs to new bookmark IDs and insert data
INSERT INTO bookmark_tags(bookmark_id, tag_id)
SELECT
    b.id,
    bt.tag_id
FROM bookmark_tags_old bt
JOIN bookmarks b
    ON b.url = bt.bookmark_url;

-- clean up legacy table
DROP TABLE bookmark_tags_old;

-- optimize lookups on the new relation
CREATE INDEX IF NOT EXISTS idx_bookmark_tags
ON bookmark_tags(bookmark_id, tag_id);
