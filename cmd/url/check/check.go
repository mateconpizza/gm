// Package check provides bookmark health verification and maintenance.
package check

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCheckCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "check",
		Short: "check URLs HTTP status",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " bookmark status ")
			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				const maxGoroutines = 15

				p := d.Console().Palette()
				q := fmt.Sprintf("checking %s of %d bookmarks", p.BrightGreen.Wrap("status", p.Bold), len(bs))
				if err := d.Console().
					ConfirmLimit(cmd.Context(), len(bs), maxGoroutines, q, app.Flags.Force); err != nil {
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
				if err := c.ConfirmLimit(cmd.Context(), len(bs), 10, s, app.Flags.Force); err != nil {
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

func NewStatusCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "filter bookmarks by HTTP status code",
		Example: `  # show all codes 4xx, 5xx
  gm url status

  # using -c, --code flag
  gm url status -c 200,400
	gm url status -c 2,4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(
				cmd,
				args,
				nil,
				func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
					return printer.Display(ctx, d.Console(), formatter.HTTPStatusCode.String(), bs)
				},
				statusCodeFilter(app.Flags.Field),
			)
		},
	}

	fields := []string{"200", "300", "400", "500"}
	c.Flags().StringVarP(&app.Flags.Field, "code", "c", "", "filter status code: "+strings.Join(fields, ", "))

	return c
}

func statusCodeFilter(code string) cmdutil.Filter {
	codes := strings.Split(strings.TrimSpace(code), ",")

	return func(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
		if len(codes) == 0 || code == "" {
			return bs
		}

		result := make([]*bookmark.Bookmark, 0, len(bs))

		for _, code := range codes {
			switch {
			// Exact status code: 200, 404, 503...
			case len(code) == 3:
				want, err := strconv.Atoi(code)
				if err != nil {
					return result
				}

				for _, b := range bs {
					if b != nil && b.HTTPStatusCode == want {
						result = append(result, b)
					}
				}

			// Status class: 2
			case len(code) == 1:
				class, err := strconv.Atoi(code)
				if err != nil {
					return result
				}

				minCode := class * 100
				maxCode := minCode + 99

				for _, b := range bs {
					if b != nil &&
						b.HTTPStatusCode >= minCode &&
						b.HTTPStatusCode <= maxCode {
						result = append(result, b)
					}
				}

			default:
				return result
			}
		}

		slices.SortFunc(result, func(a, b *bookmark.Bookmark) int {
			return cmp.Compare(a.ID, b.ID)
		})

		return result
	}
}
