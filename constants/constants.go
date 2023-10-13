package constants

import (
	"os"
)

var DBName string = "bookmarks.db"
var ConfigHome string = os.Getenv("XDG_CONFIG_HOME")
var AppName = "GoBookmarks"
