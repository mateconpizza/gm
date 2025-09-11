package db

import "fmt"

// tables.
const (
	tableMainName     = "bookmarks"
	tableTagsName     = "tags"
	tableRelationName = "bookmark_tags"
	tableTempName     = "temp_bookmarks"
)

type Schema struct {
	Name    Table
	SQL     string
	Trigger []string
	Index   string
}

type DatabaseSchemas struct {
	Main     Schema
	Tags     Schema
	Relation Schema
	Temp     Schema
}

var Schemas = DatabaseSchemas{
	Main:     schemaMain,
	Tags:     schemaTags,
	Relation: schemaRelation,
	Temp:     schemaTemp,
}

// schemaMain is the schema for the main table.
var schemaMain = Schema{
	Name:    tableMainName,
	SQL:     fmt.Sprintf(tableMainSchema, tableMainName),
	Index:   tableMainIndex,
	Trigger: []string{tableMainTriggerUpdateAt},
}

// schemaTags is the schema for the tags table.
var schemaTags = Schema{
	Name:  tableTagsName,
	SQL:   tableTagsSchema,
	Index: tableTagsIndex,
}

// schemaRelation is the schema for the relation table.
var schemaRelation = Schema{
	Name:  tableRelationName,
	SQL:   tableRelationSchema,
	Index: tableRelationIndex,
}

// schemaTemp is used for reordering the IDs in the main table.
var schemaTemp = Schema{
	Name:  tableTempName,
	SQL:   fmt.Sprintf(tableMainSchema, tableTempName),
	Index: tableMainIndex,
}

// main table.
const (
	tableMainSchema = `
		CREATE TABLE IF NOT EXISTS %s (
				id          				INTEGER PRIMARY KEY AUTOINCREMENT,
				url         				TEXT    NOT NULL UNIQUE,
				title       				TEXT    DEFAULT "",
				desc        				TEXT    DEFAULT "",
				notes        				TEXT    DEFAULT "",
				created_at  				TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				last_visit  				TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at  				TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				visit_count 				INTEGER DEFAULT 0,
				favorite    				BOOLEAN DEFAULT FALSE,
				favicon_url 				TEXT    DEFAULT "",
				favicon_local 			TEXT DEFAULT "",
				archive_url 				TEXT DEFAULT "",
				archive_timestamp 	TEXT DEFAULT "",
				checksum    				TEXT NOT NULL,
				last_checked  			TEXT DEFAULT "",
				status_code  				INTEGER DEFAULT 0,
				status_text  				TEXT DEFAULT "",
				is_active  					BOOLEAN DEFAULT TRUE
		);
	`

	tableMainIndex = `
    CREATE UNIQUE INDEX IF NOT EXISTS idx_bookmarks_url ON bookmarks(url);
		CREATE INDEX idx_bookmarks_status ON bookmarks(is_active, status_code);
		CREATE INDEX idx_bookmarks_last_checked ON bookmarks(last_checked);
	`

	tableMainTriggerUpdateAt = `
		CREATE TRIGGER IF NOT EXISTS update_bookmark_updated_at
		AFTER UPDATE ON bookmarks
		FOR EACH ROW
		BEGIN
				UPDATE bookmarks SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
		END;`
)

// tags table.
const (
	tableTagsSchema = `
    CREATE TABLE IF NOT EXISTS tags (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        name        TEXT    NOT NULL UNIQUE
    );`

	tableTagsIndex = `
    CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_name
    ON tags(name);`
)

// relation table.
const (
	tableRelationSchema = `
    CREATE TABLE IF NOT EXISTS bookmark_tags (
        bookmark_url TEXT NOT NULL,
        tag_id      INTEGER NOT NULL,
        FOREIGN KEY (bookmark_url) REFERENCES bookmarks(url) ON DELETE CASCADE,
        FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE,
        PRIMARY KEY (bookmark_url, tag_id)
    );`

	tableRelationIndex = `
    CREATE INDEX IF NOT EXISTS idx_bookmark_tags
    ON bookmark_tags(bookmark_url, tag_id);`
)
