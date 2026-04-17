// Package check provides bookmark health verification and maintenance.
package check

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c"},
		Short:   "check URLs HTTP status",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" bookmark status "),
				menu.WithPreview(app.PreviewCmd(app.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				const maxGoroutines = 15

				n := len(bs)
				if n == 0 {
					return db.ErrRecordQueryNotProvided
				}

				s := fmt.Sprintf("checking status of %d bookmarks", n)
				if err := d.Console().ConfirmLimit(n, maxGoroutines, s, d.App.Flags.Force); err != nil {
					return sys.ErrActionAborted
				}

				if err := status.Check(d.Context(), d.Console(), bs); err != nil {
					return err
				}

				for i := range bs {
					b := bs[i]
					if b.HTTPStatusCode == http.StatusTooManyRequests {
						continue
					}

					if err := d.DB.UpdateOne(d.Context(), b); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	c.AddCommand(newUpdateCmd(app))

	return c
}

func newUpdateCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "update [id|query]",
		Short: "update metadata (title|desc|tags)",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" update metadata "),
				menu.WithPreview(app.PreviewCmd(app.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				p := d.Console().Palette()
				n := len(bs)
				if n > 1 {
					d.Console().Frame().Reset().Headerln(p.Yellow.Sprintf("Updating %d bookmarks", n)).Rowln().Flush()
				}

				for _, b := range bs {
					if err := handler.ProcessBookmarkUpdate(d, b); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	return c
}
