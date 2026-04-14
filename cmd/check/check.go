// Package check provides bookmark health verification and maintenance.
package check

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/status"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c"},
		Short:   "check URLs HTTP status",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" bookmark status "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
				const maxGoroutines = 15

				n := len(bs)
				if n == 0 {
					return db.ErrRecordQueryNotProvided
				}

				s := fmt.Sprintf("checking status of %d bookmarks", n)
				if err := a.Console().ConfirmLimit(n, maxGoroutines, s, a.Cfg.Flags.Force); err != nil {
					return sys.ErrActionAborted
				}

				if err := status.Check(a.Context(), a.Console(), bs); err != nil {
					return err
				}

				for i := range bs {
					b := bs[i]
					if b.HTTPStatusCode == http.StatusTooManyRequests {
						continue
					}

					if err := a.DB.UpdateOne(a.Context(), b); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)

	c.AddCommand(newUpdateCmd(cfg))

	return c
}

func newUpdateCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "update [id|query]",
		Short: "update metadata (title|desc|tags)",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" update metadata "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, handler.Update)
		},
	}

	return c
}
