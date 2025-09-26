package config

import "path/filepath"

// App is the default application configuration.
var App = &AppConfig{
	Name:   appName,
	Cmd:    command,
	DBName: MainDBName,
	Flags:  &Flags{},
	Info: &Information{
		URL:     "https://github.com/mateconpizza/gm#readme",
		Title:   "Gomarks: A bookmark manager",
		Tags:    "golang,awesome,bookmarks,cli",
		Desc:    "Simple yet powerful bookmark manager for your terminal",
		Version: Version,
	},
	Path: &Path{},
	Git:  &Git{},
	Env: &Env{
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
