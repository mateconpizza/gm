package io

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func newExportCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "export [id|query]",
		Aliases: []string{"e", "ext"},
		Short:   "export selected bookmarks",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
			)

			return base.RunWithBookmarks(cmd, args, m, handler.Export)
		},
	}

	base.FlagsFilter(c, cfg)
	base.FlagMenu(c, cfg)

	return c
}
