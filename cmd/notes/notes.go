package notes

import (
	"errors"
	"fmt"
	"strings"

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

var ErrNotesNotFound = errors.New("notes not found")

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "notes [query]",
		Aliases: []string{"n"},
		Short:   "view/edit notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Flags.Edit {
				return newEditNotesCmd(app).RunE(cmd, args)
			}

			p := "{+1}"
			kb := menu.NewBindBuilder(app.Cmd, app.DBName).WithPlaceholder(p)
			k := app.Menu.DefaultKeymaps.Edit
			k.Enabled = true

			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "notes", strings.ReplaceAll(p, "+", ""))),
				menu.WithKeybinds(kb.New(k.Bind, k.Desc).Execute("notes --edit")),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				if len(bs) == 0 {
					return fmt.Errorf("%w: %v", ErrNotesNotFound, strings.Join(args, ""))
				}

				return printer.Notes(d.Console(), bs)
			}, OnlyNotes)
		},
	}

	cmdutil.FlagMenu(c, app)
	c.Flags().BoolVarP(&app.Flags.Edit, "edit", "e", false, "edit with text editor")
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
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "notes", "{1}")),
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
