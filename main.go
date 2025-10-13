package main

import (
	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/db"
)

func main() {
	defer db.Shutdown()
	app.InitPaths()
	config.Set(app)

	root := cmd.NewRootCmd(app)
	cmd.Setup(root)

	if err := cmd.Execute(root); err != nil {
		sys.ErrAndExit(err)
	}
}
