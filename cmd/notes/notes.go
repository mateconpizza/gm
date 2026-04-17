package notes

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "notes [query]",
		Aliases: []string{"n"},
		Short:   "view/edit notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewKeybindBuilder(app.Cmd, app.DBName)
			edit := app.Menu.DefaultKeymaps.Edit
			edit.Hidden = false

			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithKeybinds(kb.EditNotes(edit)),
				menu.WithPreview(app.PreviewCmd(app.DBName, "notes")+" {+1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				return printer.Notes(d.Console(), bs)
			}, OnlyNotes)
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	c.AddCommand(newEditNotesCmd(app))

	return c
}

func newEditNotesCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "edit [query]",
		Short: "edit notes with text editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewKeybindBuilder(app.Cmd, app.DBName)
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithKeybinds(kb.EditNotes(app.Menu.DefaultKeymaps.Edit)),
				menu.WithPreview(app.PreviewCmd(app.DBName, "notes")+" {+1}"),
			)

			return cmdutil.Execute(cmd, args, m, handler.Edit(editor.NotesStrategy{}))
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	return c
}

func OnlyNotes(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	filtered := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		if bs[i].Notes == "" {
			continue
		}

		filtered = append(filtered, bs[i])
	}

	return filtered
}
