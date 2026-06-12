package open

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open in browser",
		Example: app.Example(`  $ {cmd} open <id> or <query>
  $ {cmd} open --menu --sort favorite
  $ {cmd} open --tag golang,awesome
  $ {cmd} open --tag golang --tag awesome`),
		Annotations: cli.SkipGitSync,
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewBindBuilder(app.Cmd, app.DBName).WithPlaceholder("{+1}")
			m := picker.New[bookmark.Bookmark](
				app,
				menu.WithMultiSelection(),
				menu.WithHeaderLabel(" open in browser "),
				menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), "{1}")),
				menu.WithKeybinds(kb.New("ctrl-o", "open-snapshot").Execute("archive open")),
			)

			return cmdutil.Execute(cmd, args, m, handler.Open)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	return c
}
