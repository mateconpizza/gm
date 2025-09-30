package main

import (
	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/cmd/create"
	"github.com/mateconpizza/gm/cmd/database"
	"github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/cmd/io"
	"github.com/mateconpizza/gm/cmd/records"
	"github.com/mateconpizza/gm/cmd/settings"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
)

var (
	// Version of the application.
	Version = "0.1.29"

	// Default app configuration.
	cfg = &config.Config{
		Name:   config.AppName,
		Cmd:    config.AppCommand,
		DBName: config.MainDBName,
		Flags:  &config.Flags{},
		Info: &config.Information{
			URL:     "https://github.com/mateconpizza/gm#readme",
			Title:   "Gomarks: A bookmark manager",
			Tags:    "golang,awesome,bookmarks,cli",
			Desc:    "Simple yet powerful bookmark manager for your terminal",
			Version: Version,
		},
		Path: &config.Path{},
		Git:  &config.Git{},
		Env: &config.Env{
			Home:   "GOMARKS_HOME",
			Editor: "GOMARKS_EDITOR",
		},
	}
)

func main() {
	config.LoadPath(cfg)

	root := cmd.NewRootCmd(cfg)
	config.SetDefault(cfg)

	cli.Register(
		create.NewCmd(),
		database.NewCmd(),
		git.NewCmd(),
		io.NewCmd(),
		records.NewCmd(),
		settings.NewCmd(),
		setup.NewCmd(),
	)

	cli.AttachTo(root)
	cli.Execute(root)
}
