package main

import (
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// version of the application.
var version = "0.1.29"

// Default app configuration.
var app = &config.Config{
	Name:   config.AppName,
	Cmd:    config.AppCommand,
	DBName: config.MainDBName,
	Flags:  &config.Flags{},
	Info: &config.Information{
		URL:     "https://github.com/mateconpizza/gm#readme",
		Title:   "Gomarks: A bookmark manager",
		Tags:    "golang,awesome,bookmarks,cli",
		Desc:    "Simple yet powerful bookmark manager for your terminal",
		Version: version,
	},
	Path: &config.Path{},
	Git: &config.Git{
		Enabled: false,
		GPG:     false,
		Log:     true, // FIX: not implemented yet
	},
	Menu: &menu.Config{},
	Env: &config.Env{
		Home:   "GOMARKS_HOME",
		Editor: "GOMARKS_EDITOR",
	},
}
