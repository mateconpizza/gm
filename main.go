package main

import (
	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	info := application.NewInfo(version, commit, date)
	app := application.New(info)
	if err := app.Setup(); err != nil {
		sys.ErrAndExit(err)
	}

	// load config from YAML
	if err := app.Load(); err != nil {
		sys.ErrAndExit(err)
	}
	defer cleanup.Run()

	root := cmd.NewRootCmd(app)
	cmd.Setup(root, app)

	if err := cmd.Execute(root); err != nil {
		sys.ErrAndExit(err)
	}
}
