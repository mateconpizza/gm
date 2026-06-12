package yank

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
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
			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				c := d.Console()
				p := c.Palette()

				msg := fmt.Sprintf(
					"%s %d bookmarks to system clipboard",
					p.BrightGreen.Wrap("copy", p.Bold),
					len(bs),
				)

				if err := c.ConfirmLimit(ctx, len(bs), 10, msg, app.Flags.Force); err != nil {
					return err
				}

				content, err := clipboardContent(bs, app.Flags.JSON)
				if err != nil {
					return err
				}

				if !app.Flags.Force && !app.Flags.Yes {
					c.ClearLine(2)
				}

				if err := sys.CopyClipboard(content); err != nil {
					return err
				}

				return c.Print(
					ctx,
					c.SuccessMesg("copied ", len(bs), " bookmarks to system clipboard\n"),
				)
			}

			return cmdutil.Execute(cmd, args, setupMenu(app), a)
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "yank as JSON")
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func clipboardContent(bs []*bookmark.Bookmark, asJSON bool) (string, error) {
	if asJSON {
		b, err := port.ToJSON(bs)
		if err != nil {
			return "", err
		}

		return string(b), nil
	}

	var sb strings.Builder
	for _, b := range bs {
		sb.WriteString(b.URL)
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}

func setupMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	return picker.New[bookmark.Bookmark](
		app,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" yank URL "),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), "{1}")+" {1}"),
	)
}
