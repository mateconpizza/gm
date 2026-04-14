package archive

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "archive [query]",
		Aliases: []string{"snap", "ar", "a"},
		Short:   "show archive URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Flags.Snapshot = true
			m := handler.MenuSimple[bookmark.Bookmark](
				cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" archive URL "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
				maxItems := 15

				n := len(bs)
				if n == 0 {
					return handler.ErrNoItems
				}

				action := func(u string) error {
					fmt.Println(u)
					return nil
				}

				if a.Cfg.Flags.Open {
					c, p := a.Console(), a.Console().Palette()

					// get user confirmation to procced
					s := fmt.Sprintf("%s %d bookmarks", p.BrightGreen.Wrap("open", p.Bold), n)
					if err := c.ConfirmLimit(n, maxItems, s, a.Cfg.Flags.Force); err != nil {
						return err
					}

					action = sys.OpenInBrowser
				}

				for _, u := range bs {
					if err := action(u.ArchiveURL); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	cmdutil.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	cmdutil.FlagsFilter(c, cfg)

	c.AddCommand(newLookupCmd(cfg))
	c.AddCommand(newOpenCmd(cfg))

	return c
}

func newOpenCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open archive URL in browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Flags.Snapshot = true

			m := handler.MenuSimple[bookmark.Bookmark](
				cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" open archive URL "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
				filtered := make([]*bookmark.Bookmark, 0, len(bs))
				for i := range bs {
					if bs[i].ArchiveURL != "" {
						filtered = append(filtered, bs[i])
					}
				}

				if len(filtered) == 0 {
					return bookmark.ErrBookmarkArchiveURL
				}

				result := make([]*bookmark.Bookmark, 0, len(filtered))
				for i := range filtered {
					b := bookmark.New()
					b.URL = filtered[i].ArchiveURL
					result = append(result, b)
				}

				return handler.Open(a, result)
			})
		},
	}

	cmdutil.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	return c
}
