package notes

import (
	"context"
	"errors"

	menu "github.com/mateconpizza/go-fzf"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrNotesNotFound = errors.New("notes not found")

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "notes [query]",
		Aliases: []string{"n"},
		Short:   "view notes",
		Example: app.Example(`  $ {cmd} notes <id> or <query>
  $ {cmd} notes --menu --sort favorite
  $ {cmd} notes --sort newest <query>
  $ {cmd} notes edit --tag golang,awesome
  $ {cmd} notes edit --tag golang --tag awesome`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fm := app.Formatter()
			p := fm.Menu.Placeholder()

			kb := menucfg.NewBindBuilder().
				WithCommand(app.Command()).
				WithDBName(app.DBBaseName()).
				WithPlaceholder(p.Multi())

			k := app.Menu.Keymaps()
			k.Edit.Enabled = true

			m := setupMenu(
				app,
				menu.WithHeaderKeymaps(),
				menu.WithKeybinds(
					kb.New(k.Edit.Bind, k.Edit.Desc).WithExecute("edit notes"),
					menu.KeymapToggleAll(),
					menu.KeymapTogglePreview(),
				),
			)

			return cmdutil.Execute(
				cmd,
				args,
				m,
				printNotes,
				handler.WithNotes,
			)
		},
	}

	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagOutput(c, app, app.Format, formatter.ValidFormats())
	c.AddCommand(newEditNotesCmd(app))

	return c
}

func newEditNotesCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "edit [query]",
		Short: "edit notes with text editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, menu.WithKeybinds(menu.KeymapTogglePreview()))
			return cmdutil.Execute(cmd, args, m, handler.Edit(cmd.Context(), editor.NewNotesStrategy()))
		},
	}

	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagOutput(c, app, app.Format, formatter.ValidFormats())

	return c
}

func printNotes(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
	if len(bs) == 0 {
		return ErrNotesNotFound
	}
	return printer.Notes(ctx, d.Console(), bs)
}

func setupMenu(app *application.App, opts ...menu.Option) *menu.Menu[bookmark.Bookmark] {
	p := app.Formatter().Menu.Placeholder()
	return picker.NewWithFormatter(app, app.Formatter(), append(
		opts,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithBorderLabel(" notes "),
		menu.WithPreviewWindow(picker.PreviewWindowArg(app.Menu.Preview)),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), app.DBBaseName(), "notes", p.Single())),
	)...)
}
