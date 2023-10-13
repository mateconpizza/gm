package main

import (
	"os"
)

const (
	DBName          string = "bookmarks.db"
	AppName         string = "GoBookmarks"
	BookmarksSquema string = `
    CREATE TABLE IF NOT EXISTS bookmarks (
        id INTEGER PRIMARY KEY,
        url TEXT NOT NULL UNIQUE,
        title TEXT DEFAULT "",
        tags TEXT DEFAULT ",",
        desc TEXT DEFAULT "",
        created_at TIMESTAMP,
        last_used TIMESTAMP
    )
  `
)

var InitBookmark Bookmark = Bookmark{
	ID:    0,
	URL:   "https://github.com/haaag/GoMarks/",
	Title: "GoMarks",
	Tags:  "golang,awesome,bookmarks",
	Desc:  "Makes accessing, adding, updating, and removing bookmarks easier",
}
var ConfigHome string = os.Getenv("XDG_CONFIG_HOME")

// gomarks -json | jq '.[].ID, .[].URL' | sed 's/"\(.*\)"/\1/'
