package main

import (
	"github.com/mateconpizza/gm/cmd"
	_ "github.com/mateconpizza/gm/cmd/git"
	_ "github.com/mateconpizza/gm/cmd/imports"
)

func main() {
	cmd.Execute()
}
