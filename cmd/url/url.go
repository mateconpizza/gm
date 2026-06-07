package urlcmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/url/archive"
	"github.com/mateconpizza/gm/cmd/url/check"
	"github.com/mateconpizza/gm/cmd/url/clean"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "url",
		Short: "URL utilities",
		RunE:  cli.HookHelp,
	}

	c.AddCommand(
		check.NewCmd(app),
		clean.NewCmd(app),
		archive.NewCmd(app),
	)

	return c
}
