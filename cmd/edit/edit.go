package edit

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// FIX: NewCmd menu: current functionality exits the menu after editing a bookmark.
// New functionality must keep menu after editing.

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "edit [query]",
		Aliases: []string{"e"},
		Short:   "edit bookmark",
		Example: app.Example(`  $ {cmd} edit <id> or <query>
  $ {cmd} edit --menu --sort favorite
  $ {cmd} edit --tag golang,awesome
  $ {cmd} edit --tag golang --json
  $ {cmd} edit --tag golang --tag awesome`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fm := app.UI.MenuFmt
			p := fm.Menu.Placeholder

			kb := menu.NewBindBuilder(app.Cmd, app.DBName).
				WithPlaceholder(p)

			k := app.Menu.DefaultKeymaps
			k.Edit.Enabled = true

			m := picker.NewWithFormatter(
				app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" edition "),
				menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), p)),
				menu.WithKeybinds(kb.New(k.Edit.Bind, "as-json").Execute("edit --json")),
				menu.WithKeybinds(kb.New(k.EditNotes.Bind, "notes").Execute("edit notes")),
			)

			var strategy editor.EditStrategy
			strategy = editor.NewBookmarkStrategy()
			if app.Flags.JSON {
				strategy = editor.NewJSONStrategy()
			}

			return cmdutil.Execute(cmd, args, m, handler.Edit(cmd.Context(), strategy))
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "JSON format")
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	c.AddCommand(newEditNotesCmd(app))

	return c
}

func newEditNotesCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "notes [query]",
		Short: "edit notes with text editor",
		Example: app.Example(`  $ {cmd} edit notes <id> or <query>
  $ {cmd} edit notes --menu --sort favorite
  $ {cmd} edit notes --tag golang,awesome
  $ {cmd} edit notes --tag golang --tag awesome`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fm := app.UI.MenuFmt
			p := fm.Menu.Placeholder
			m := picker.NewWithFormatter(
				app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), "notes", p)),
			)
			return cmdutil.Execute(cmd, args, m, handler.Edit(cmd.Context(), editor.NewNotesStrategy()))
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagsFilter(c, app)

	return c
}
