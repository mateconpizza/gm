package qrcmd

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
		Use:     "qr [query]",
		Aliases: []string{"q"},
		Short:   "generate QR",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
			)
			return base.Execute(cmd, args, m, handler.QR)
		},
	}

	base.FlagMenu(c, cfg)
	c.Flags().BoolVarP(&cfg.Flags.Open, "open", "o", false, "open in display")
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagsFilter(c, cfg)

	return c
}
