package main

import (
	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
)

var info = &application.Information{
	URL:     "https://github.com/mateconpizza/gm#readme",
	Title:   "Gomarks: A bookmark manager",
	Tags:    "awesome,bookmarks,cli,golang",
	Desc:    "Simple yet powerful bookmark manager for your terminal",
	Version: "0.1.35",
}

func main() {
	app := application.New(info)
	if err := app.Setup(); err != nil {
		sys.ErrAndExit(err)
	}
	defer cleanup.Run()

	root := cmd.NewRootCmd(app)
	cmd.Setup(root, app)

	if err := cmd.Execute(root); err != nil {
		sys.ErrAndExit(err)
	}
}
