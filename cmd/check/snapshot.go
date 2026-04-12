package check

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func newSnapCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "snapshot [query]",
		Aliases: []string{"snap", "archive"},
		Short:   "display archive URL",
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

	c.AddCommand(newSnapOpenCmd(cfg))

	return c
}

func newSnapOpenCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open bookmark archive URL in default browser",
		RunE:    snapOpenFunc,
	}

	base.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	return c
}

func snapOpenFunc(cmd *cobra.Command, args []string) error {
	a, cleanup, err := base.SetupApp(cmd, &args)
	if err != nil {
		return err
	}
	defer cleanup()

	bs, err := handler.FetchBookmarks(a, args)
	if err != nil {
		return err
	}

	filtered := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		if bs[i].ArchiveURL != "" {
			filtered = append(filtered, bs[i])
		}
	}

	if len(filtered) == 0 {
		return bookmark.ErrBookmarkArchiveURL
	}

	if a.Cfg.Flags.Menu {
		m := handler.MenuSimple[bookmark.Bookmark](a.Cfg,
			menu.WithMultiSelection(),
		)
		filtered, err = handler.ApplyMenuSelection(a.Console(), m, filtered)
		if err != nil {
			return err
		}
	}

	result := make([]*bookmark.Bookmark, 0, len(filtered))
	for i := range filtered {
		b := bookmark.New()
		b.URL = filtered[i].ArchiveURL
		result = append(result, b)
	}

	return handler.Open(a, result)
}
