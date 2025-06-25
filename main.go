package main

import (
	"github.com/mateconpizza/gm/cmd"
	_ "github.com/mateconpizza/gm/cmd/create"
	_ "github.com/mateconpizza/gm/cmd/db"
	_ "github.com/mateconpizza/gm/cmd/git"
	_ "github.com/mateconpizza/gm/cmd/imports"
)

func main() {
	cmd.Execute()
}
