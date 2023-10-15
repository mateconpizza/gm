package main

import (
	"database/sql"
	"os"
)

const (
	DBName          string = "bookmarks.db"
	AppName         string = "GoBookmarks"
	BookmarksSquema string = `
    CREATE TABLE IF NOT EXISTS bookmarks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url TEXT NOT NULL UNIQUE,
        title TEXT DEFAULT "",
        tags TEXT DEFAULT ",",
        desc TEXT DEFAULT "",
        created_at TIMESTAMP,
        last_used TIMESTAMP
    )
  `
	DeletedBookmarksSchema string = `
    CREATE TABLE IF NOT EXISTS deleted (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url TEXT NOT NULL UNIQUE,
        title TEXT DEFAULT "",
        tags TEXT DEFAULT ",",
        desc TEXT DEFAULT "",
        deleted_at TIMESTAMP
    )
  `
)

var InitBookmark Bookmark = Bookmark{
	ID:    0,
	URL:   "https://github.com/haaag/GoMarks/",
	Title: NullString{sql.NullString{String: "GoMarks", Valid: true}},
	Tags:  "golang,awesome,bookmarks",
	Desc: NullString{
		sql.NullString{
			String: "Makes accessing, adding, updating, and removing bookmarks easier",
			Valid:  true,
		},
	},
}
var ConfigHome string = os.Getenv("XDG_CONFIG_HOME")

// gomarks -json | jq '.[].ID, .[].URL' | sed 's/"\(.*\)"/\1/'
