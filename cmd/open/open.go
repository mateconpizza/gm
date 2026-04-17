package open

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
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open in browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewKeybindBuilder(app.Cmd, app.DBName)
			k := menu.NewKeymap()
			k = k.WithSilentAction(kb.BaseCmd("archive open") + " {1}")
			k.Bind = "ctrl-o"
			k.Desc = "open-snapshot"
			k.Enabled = true

			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeaderLabel(" open in browser "),
				menu.WithPreview(app.PreviewCmd(app.DBName)+" {1}"),
				menu.WithKeybinds(k),
			)

			return cmdutil.Execute(cmd, args, m, handler.Open)
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	return c
}
