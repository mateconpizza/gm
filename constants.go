package main

import (
	"database/sql"
	"fmt"
	"os"
)

const (
	DBName         string = "bookmarks.db"
	DBMainTable    string = "bookmarks"
	DBDeletedTable string = "deleted_bookmarks"
	AppName        string = "GoMarks"
)

var (
	ConfigHome   string   = os.Getenv("XDG_CONFIG_HOME")
	InitBookmark Bookmark = Bookmark{
		ID:    0,
		URL:   "https://github.com/haaag/GoMarks#readme",
		Title: NullString{sql.NullString{String: "GoMarks", Valid: true}},
		Tags:  "golang,awesome,bookmarks",
		Desc: NullString{
			sql.NullString{
				String: "Makes accessing, adding, updating, and removing bookmarks easier",
				Valid:  true,
			},
		},
	}
	NoBookmarkFound Bookmark = Bookmark{
		ID:    0,
		URL:   "No bookmarks found",
		Title: NullString{sql.NullString{String: "No bookmarks found", Valid: false}},
		Tags:  "",
		Desc: NullString{
			sql.NullString{
				String: "",
				Valid:  false,
			},
		},
	}
	BookmarksSquema string = fmt.Sprintf(`
    CREATE TABLE IF NOT EXISTS %s (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        url         TEXT    NOT NULL UNIQUE,
        title       TEXT    DEFAULT "",
        tags        TEXT    DEFAULT ",",
        desc        TEXT    DEFAULT "",
        created_at  TIMESTAMP
    )
  `, DBMainTable)
	DeletedBookmarksSchema string = fmt.Sprintf(`
    CREATE TABLE IF NOT EXISTS %s (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url         TEXT    NOT NULL UNIQUE,
        title       TEXT    DEFAULT "",
        tags        TEXT    DEFAULT ",",
        desc        TEXT    DEFAULT "",
        created_at  TIMESTAMP
    )
  `, DBDeletedTable)
	TempBookmarksSchema string = fmt.Sprintf(`
    CREATE TABLE IF NOT EXISTS temp_%s (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        url         TEXT    NOT NULL UNIQUE,
        title       TEXT    DEFAULT "",
        tags        TEXT    DEFAULT ",",
        desc        TEXT    DEFAULT "",
        created_at  TIMESTAMP
    )
  `, DBMainTable)
)

// gomarks -json | jq '.[].ID, .[].URL' | sed 's/"\(.*\)"/\1/'
