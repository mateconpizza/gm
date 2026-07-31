package edit

import (
	menu "github.com/mateconpizza/go-fzf"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
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
			fm := app.Formatter()
			p := fm.Menu.Placeholder()

			kb := menucfg.NewBindBuilder(app.Cmd, app.DBName).
				WithPlaceholder(p.Multi())

			k := app.Menu.Keymaps()
			k.Edit.Enabled = true

			keys := []*menu.Keymap{
				kb.New(k.Edit.Bind, "as-json").Execute("edit --json"),
				kb.New(k.EditNotes.Bind, "notes").Execute("edit notes"),
				kb.Builtin(k.ToggleAll, menucfg.ToggleAll),
				kb.Builtin(k.Preview, menucfg.TogglePreview),
				menu.NewKeymap().WithBind(menu.KeyTab).WithDesc("toggle-select"),
			}

			fm.Menu.Opts = append(
				fm.Menu.Opts,
				menu.WithMultiSelection(),
				menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), p.Single())),
				menu.WithKeybinds(keys...),
				// header
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" edition "),
				menu.WithHeaderKeymaps(),
				menu.WithHeaderKeymapFmt(picker.HeaderKeymapFmt),
				menu.WithHeaderSeparatorFmt(picker.HeaderSeparatorFmt),
			)

			m := picker.NewWithFormatter(app, fm)

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
			fm := app.Formatter()
			p := fm.Menu.Placeholder()
			m := picker.NewWithFormatter(
				app,
				fm,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), "notes", p.Single())),
			)
			return cmdutil.Execute(cmd, args, m, handler.Edit(cmd.Context(), editor.NewNotesStrategy()))
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagsFilter(c, app)

	return c
}
