package rm

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm [query]",
		Aliases: []string{"remove"},
		Short:   "remove bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](
				app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" deletion "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
			)

			return cmdutil.Execute(cmd, args, m, handler.Remove)
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	return c
}
