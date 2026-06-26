package urlcmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/url/archive"
	"github.com/mateconpizza/gm/cmd/url/check"
	"github.com/mateconpizza/gm/cmd/url/clean"
	"github.com/mateconpizza/gm/internal/application"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "url",
		Short: "URL operations",
	}

	c.AddCommand(
		check.NewCheckCmd(app),
		check.NewStatusCmd(app),
		clean.NewCmd(app),
		archive.NewCmd(app),
	)

	return c
}
