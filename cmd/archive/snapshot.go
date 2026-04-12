package archive

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
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
			m := handler.MenuSimple[bookmark.Bookmark](cfg, menu.WithMultiSelection())
			return base.RunWithBookmarks(cmd, args, m, handler.Snapshot)
		},
	}

	base.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagsFilter(c, cfg)

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

			m := handler.MenuSimple[bookmark.Bookmark](cfg, menu.WithMultiSelection())

			return base.RunWithBookmarks(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
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

				for i := range result {
					fmt.Printf("result[i].URL: %v\n", result[i].URL)
				}

				return handler.Open(a, result)
			})
		},
	}

	base.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	return c
}
