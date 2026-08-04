package open

import (
	menu "github.com/mateconpizza/go-fzf"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/ui/formatter"
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
			fm := app.Formatter()
			p := fm.Menu.Placeholder()

			kb := menucfg.NewBindBuilder().
				WithCommand(app.Command()).
				WithDBName(app.DBBaseName()).
				WithPlaceholder(p.Multi())

			m := picker.NewWithFormatter(
				app,
				fm,
				menu.WithMultiSelection(),
				menu.WithHeaderKeymaps(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" open in browser "),
				menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
				menu.WithKeybinds(
					kb.New(menu.KeyCtrlO, "open-snapshot").WithExecute("archive open"),
					menu.KeymapTogglePreview(),
				),
			)

			return cmdutil.Execute(cmd, args, m, handler.Open)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagOutput(c, app, app.Format, formatter.ValidFormats())
	return c
}
