package urlcmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/archive"
	"github.com/mateconpizza/gm/cmd/check"
	"github.com/mateconpizza/gm/cmd/clean"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "url",
		Short:   "URL utilities",
		Aliases: []string{},
		RunE:    cli.HookHelp,
	}

	c.AddCommand(check.NewCmd(app), clean.NewCmd(app), archive.NewCmd(app))

	return c
}
