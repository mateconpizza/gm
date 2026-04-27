package yank

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "yank [query]",
		Aliases: []string{"copy", "y"},
		Short:   "copy URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" yank URL "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				t, p := d.Console(), d.Console().Palette()
				s := fmt.Sprintf("%s %d bookmarks to system clipboard", p.BrightGreen.Wrap("copy", p.Bold), len(bs))
				if err := t.ConfirmLimit(len(bs), 10, s, app.Flags.Force); err != nil {
					return err
				}

				var sb strings.Builder
				for i := range bs {
					sb.WriteString(bs[i].URL + "\n")
				}
				if err := sys.CopyClipboard(sb.String()); err != nil {
					return err
				}

				fmt.Println(t.SuccessMesg("copied ", len(bs), " bookmarks to system clipboard"))

				return nil
			})
		},
	}

	return c
}
