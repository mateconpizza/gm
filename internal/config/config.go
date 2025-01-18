package config

var Version string = "0.1.3" // Version of the application

const (
	AppName string = "gomarks"      // Default name of the application
	Command string = "gm"           // Default name of the executable
	DBName  string = "bookmarks.db" // Default name of the database
)

type Table string

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
	Tables           tables // Names of the tables in the SQLite database.
	BackupMaxBackups int    // Maximum number of backups allowed.
}

type tables struct {
	Main Table
}

// DB is the default database configuration.
var DB = database{
	Name:             "bookmarks.db",
	Tables:           tables{Main: "bookmarks"},
	BackupMaxBackups: 3,
}

// App is the default application configuration.
var App = app{
	Name:    AppName,
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
