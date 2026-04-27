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
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c"},
		Short:   "check URLs HTTP status",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " bookmark status ")
			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				const maxGoroutines = 15

				s := fmt.Sprintf("checking status of %d bookmarks", len(bs))
				if err := d.Console().ConfirmLimit(len(bs), maxGoroutines, s, d.App.Flags.Force); err != nil {
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

					if err := d.Repo.UpdateOne(d.Context(), b); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	c.AddCommand(newUpdateCmd(app))

	return c
}

func newUpdateCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "update [id|query]",
		Short: "update metadata: title, desc, tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " update metadata ")
			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				c, p := d.Console(), d.Console().Palette()

				s := fmt.Sprintf("update metadata of %d bookmarks", len(bs))
				if err := c.ConfirmLimit(len(bs), 10, s, d.App.Flags.Force); err != nil {
					return sys.ErrActionAborted
				}

				if len(bs) > 1 {
					c.Frame().Reset().Headerln(p.Yellow.Sprintf("Updating %d bookmarks", len(bs))).Rowln().Flush()
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

func setupMenu(app *application.App, label string) *menu.Menu[bookmark.Bookmark] {
	return handler.MenuSimple[bookmark.Bookmark](app,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(label),
		menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
	)
}
