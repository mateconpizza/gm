package main

import (
	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
)

func main() {
	app.InitPaths()
	config.Set(app)

	root := cmd.NewRootCmd(app)
	cmd.Setup(root)
	cmd.Execute(root)
}
