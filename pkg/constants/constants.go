package constants

import (
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
