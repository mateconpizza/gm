// Package check provides bookmark health verification and maintenance.
package check

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "status",
		Aliases: []string{"sta", "s"},
		Short:   "check URLs HTTP status",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " bookmark status ")
			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				const maxGoroutines = 15

				p := d.Console().Palette()
				q := fmt.Sprintf("checking %s of %d bookmarks", p.BrightGreen.Wrap("status", p.Bold), len(bs))
				if err := d.Console().ConfirmLimit(len(bs), maxGoroutines, q, app.Flags.Force); err != nil {
					return sys.ErrActionAborted
				}

				if err := status.Check(cmd.Context(), d.Console(), bs); err != nil {
					return err
				}

				r, err := d.Repository()
				if err != nil {
					return err
				}

				for i := range bs {
					b := bs[i]
					if b.HTTPStatusCode == http.StatusTooManyRequests {
						continue
					}

					if err := r.UpdateOne(ctx, b); err != nil {
						return err
					}
				}

				return nil
			}

			return cmdutil.Execute(cmd, args, m, a)
		},
	}

	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	c.AddCommand(newUpdateCmd(app))

	return c
}

func newUpdateCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "update [id|query]",
		Short: "update metadata: title, desc, tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " update metadata ")
			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				c, p := d.Console(), d.Console().Palette()

				s := fmt.Sprintf("update metadata of %d bookmarks", len(bs))
				if err := c.ConfirmLimit(len(bs), 10, s, app.Flags.Force); err != nil {
					return sys.ErrActionAborted
				}

				if len(bs) > 1 {
					c.Frame().Reset().Headerln(p.Yellow.Sprintf("Updating %d bookmarks", len(bs))).Rowln().Flush()
				}

				for _, b := range bs {
					if err := handler.ProcessBookmarkUpdate(cmd.Context(), d, b); err != nil {
						return err
					}
				}

				return nil
			}

			return cmdutil.Execute(cmd, args, m, a)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	return c
}

func setupMenu(app *application.App, label string) *menu.Menu[bookmark.Bookmark] {
	return picker.New[bookmark.Bookmark](
		app,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(label),
		menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
	)
}
