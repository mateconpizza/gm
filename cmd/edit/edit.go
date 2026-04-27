package edit

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "edit [query]",
		Aliases: []string{"e"},
		Short:   "edit bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			// FIX: menu: current functionality exits the menu after editing a bookmark.
			// New functionality must keep menu after editing.

			// FIX: update `placeholder`

			kb := menu.NewBindBuilder(app.Cmd, app.DBName).WithPlaceholder("{+1}")
			k := app.Menu.DefaultKeymaps
			k.Edit.Enabled = true
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" edition "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
				menu.WithKeybinds(kb.New(k.Edit.Bind, "as-json").Execute("edit --json")),
			)

			var strategy editor.EditStrategy
			strategy = editor.BookmarkStrategy{}
			if app.Flags.JSON {
				strategy = editor.JSONStrategy{}
			}

			return cmdutil.Execute(cmd, args, m, handler.Edit(strategy))
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "edit bookmark as JSON")

	return c
}
