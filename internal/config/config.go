package config

import "os"

const (
	appName string = "gomarks" // Default name of the application
	Command string = "gm"      // Default name of the executable
)

var Version string = "0.0.9" // Version of the application

type (
	app struct {
		Name    string      `json:"name"`    // Name of the application
		Cmd     string      `json:"cmd"`     // Name of the executable
		Version string      `json:"version"` // Version of the application
		Info    information `json:"data"`    // Application information
		Env     environment `json:"env"`     // Application environment variables
		Path    path        `json:"path"`    // Application path
		Color   bool        `json:"-"`       // Application color enable
	}

	path struct {
		Backup string `json:"backup"` // Path to store database backups
		Config string `json:"home"`   // Path to store configuration (unused)
		Data   string `json:"data"`   // Path to store database
	}

	information struct {
		URL   string `json:"url"`   // URL of the application
		Title string `json:"title"` // Title of the application
		Tags  string `json:"tags"`  // Tags of the application
		Desc  string `json:"desc"`  // Description of the application
	}

	environment struct {
		Home      string `json:"home"`        // Environment variable for the home directory
		Editor    string `json:"editor"`      // Environment variable for the preferred editor
		BackupMax string `json:"max_backups"` // Environment variable for the maximum number of backups
	}
)

type database struct {
	Name             string // Default name of the SQLite database.
	MainTable        string // Name of the main bookmarks table.
	DeletedTable     string // Name of the deleted bookmarks table.
	DateFormat       string // Database date format
	BackupDateFormat string // Database backup date format
	MaxBytesSize     int64  // Maximum size in bytes of the SQLite database before vacuum.
	BackupMaxBackups int    // Maximum number of backups allowed.
}

type files struct {
	DirPermissions  os.FileMode // Permissions for new directories.
	FilePermissions os.FileMode // Permissions for new files.
}

// DB is the default database configuration.
var DB = database{
	Name:             "bookmarks.db",
	MainTable:        "bookmarks",
	DeletedTable:     "deleted_bookmarks",
	DateFormat:       "2006-01-02 15:04:05",
	BackupDateFormat: "2006-01-02_15-04",
	MaxBytesSize:     1000000,
	BackupMaxBackups: 3,
}

// Files is the default files permissions.
var Files = files{
	DirPermissions:  0o755,
	FilePermissions: 0o644,
}

// App is the default application configuration.
var App = app{
	Name:    appName,
	Cmd:     Command,
	Version: Version,
	Info: information{
		URL:   "https://github.com/haaag/gomarks#readme",
		Title: "Gomarks: A bookmark manager",
		Tags:  "golang,awesome,bookmarks,cli",
		Desc:  "Simple yet powerful bookmark manager for your terminal",
	},
	Env: environment{
		Home:      "GOMARKS_HOME",
		Editor:    "GOMARKS_EDITOR",
		BackupMax: "GOMARKS_BACKUP_MAX",
	},
}
