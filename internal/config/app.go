package config

import "path/filepath"

type Flags struct {
	Copy      bool     // Copy URL into clipboard
	Open      bool     // Open URL in default browser
	Tags      []string // Tags list to filter bookmarks
	QR        bool     // QR code generator
	Menu      bool     // Menu mode
	Edit      bool     // Edit mode
	Head      int      // Head limit
	Remove    bool     // Remove bookmarks
	Update    bool     // Update bookmarks
	Tail      int      // Tail limit
	Field     string   // Field to print
	JSON      bool     // JSON output
	Oneline   bool     // Oneline output
	Multiline bool     // Multiline output
	ColorStr  string   // WithColor enable color output
	Color     bool     // Application color enable
	Force     bool     // Force action
	Status    bool     // Status checks URLs status code
	Verbose   int      // Verbose flag
}

// App is the default application configuration.
var App = &AppConfig{
	Name:   appName,
	Cmd:    command,
	DBName: MainDBName,
	Flags:  &Flags{},
	Info: information{
		URL:     "https://github.com/mateconpizza/gm#readme",
		Title:   "Gomarks: A bookmark manager",
		Tags:    "golang,awesome,bookmarks,cli",
		Desc:    "Simple yet powerful bookmark manager for your terminal",
		Version: version,
	},
	Env: environment{
		Home:   "GOMARKS_HOME",
		Editor: "GOMARKS_EDITOR",
	},
}

// SetAppPaths sets the app data path.
func SetAppPaths(p string) {
	App.Path.Data = p
	App.Path.ConfigFile = filepath.Join(p, configFilename)
	App.Path.Backup = filepath.Join(p, "backup")
	App.Git.Path = filepath.Join(p, "git")
}
