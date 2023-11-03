package constants

import (
	"os"
	"strings"
)

const (
	DBName             string = "bookmarks.db"
	DBMainTableName    string = "bookmarks"
	DBDeletedTableName string = "deleted_bookmarks"
	DBTempTableName    string = "temp_bookmarks"
	AppName            string = "gomarks"
	BulletPoint        string = "\u2022"
	Version            string = "0.0.2"
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

var AppHelp = strings.TrimSpace(`
Gomarks is a bookmark manager for your terminal.

Usage:
  gomarks               show all bookmarks
  gomarks   <query>     query to filter bookmarks

Options:
  -id       <number>    select bookmark by id
  -head     <number>    output the <number> first part of bookmarks
  -tail     <number>    output the <number> last part of bookmarks 
  -format   <option>    output format [json | pretty | plain] (default: pretty)
  -oneline  <option>    pick oneline data [url | title | tags]
  -menu     <option>    menu mode [dmenu | rofi]

Additional Options:
  -add                  add bookmark with tags
  -edit                 edit selected bookmark
  -delete               delete selected bookmark
  -copy                 copy to system clipboar (default)
  -open                 open in default browser
  -version              show version
  -info                 show app info
`)
