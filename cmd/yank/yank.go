package yank

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "yank [query]",
		Aliases: []string{"copy", "y"},
		Short:   "copy URL",
		Example: app.Example(`  $ {cmd} yank <query>
  $ {cmd} yank --menu
  $ {cmd} yank --menu --sort favorite
  $ {cmd} yank --tag golang
  $ {cmd} yank --tag golang,awesome
  $ {cmd} yank --json <query>`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(cmd, args, setupMenu(app), handler.Yank)
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "yank as JSON")
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func setupMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	keys := app.Menu.DefaultKeymaps
	keys.Yank.Hidden = false

	fm := app.MenuFormatter()
	p := fm.Menu.Placeholder()
	kb := menu.NewBindBuilder(app.Cmd, app.DBName).
		WithPlaceholder(p.Multi())

	return picker.NewWithFormatter(
		app,
		fm,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" yank URL "),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
		menu.WithKeybinds(kb.From(keys.Yank).Execute("yank")),
	)
}
