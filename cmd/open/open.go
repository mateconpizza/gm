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
			p := "{+1}"
			kb := menu.NewBindBuilder(app.Cmd, app.DBName).WithPlaceholder(p)
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeaderLabel(" open in browser "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
				menu.WithKeybinds(kb.New("ctrl-o", "open-snapshot").ExecuteSilent("archive open")),
			)

			return cmdutil.Execute(cmd, args, m, handler.Open)
		},
	}

	return c
}
