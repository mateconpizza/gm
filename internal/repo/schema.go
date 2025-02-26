package repo

var (
	tableMainSchema = `
    CREATE TABLE IF NOT EXISTS %s (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        url         TEXT    NOT NULL UNIQUE,
        title       TEXT    DEFAULT "",
        desc        TEXT    DEFAULT "",
        created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        last_visit  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        visit_count INTEGER DEFAULT 0,
        updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        favorite    BOOLEAN DEFAULT FALSE
    );`

	tableTagsSchema = `
    CREATE TABLE IF NOT EXISTS %s (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        name        TEXT    NOT NULL UNIQUE
    );`

	tableBookmarkTagsSchema = `
    CREATE TABLE IF NOT EXISTS %s (
        bookmark_url TEXT NOT NULL,
        tag_id      INTEGER NOT NULL,
        FOREIGN KEY (bookmark_url) REFERENCES bookmarks(id),
        FOREIGN KEY (tag_id) REFERENCES tags(id),
        PRIMARY KEY (bookmark_url, tag_id)
    );`
)
