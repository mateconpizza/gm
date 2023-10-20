package constants

import (
	"os"
)

const (
	DBName             string = "bookmarks.db"
	DBMainTableName    string = "bookmarks"
	DBDeletedTableName string = "deleted_bookmarks"
	DBTempTableName    string = "temp_bookmarks"
	AppName            string = "GoMarks"
)

var (
	ConfigHome      string = os.Getenv("XDG_CONFIG_HOME")
	MainTableSchema string = `
    CREATE TABLE IF NOT EXISTS %s (
        id          INTEGER PRIMARY KEY AUTOINCREMENT,
        url         TEXT    NOT NULL UNIQUE,
        title       TEXT    DEFAULT "",
        tags        TEXT    DEFAULT ",",
        desc        TEXT    DEFAULT "",
        created_at  TIMESTAMP
    )
  `
)
