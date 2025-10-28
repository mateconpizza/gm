package main

import (
	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
)

// version of the application.
var version = "0.1.30"

func main() {
	cfg := config.NewDefaultConfig(version)
	cfg.InitPaths()

	root := cmd.NewRootCmd(cfg)
	cmd.Setup(root, cfg)

	if err := cmd.Execute(root); err != nil {
		sys.ErrAndExit(err)
	}
}
