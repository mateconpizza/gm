// Package check provides bookmark health verification and maintenance.
package check

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
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

			return base.Execute(cmd, args, m, handler.CheckStatus)
		},
	}

	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagMenu(c, cfg)
	base.FlagsFilter(c, cfg)

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

			return base.Execute(cmd, args, m, handler.Update)
		},
	}

	return c
}
