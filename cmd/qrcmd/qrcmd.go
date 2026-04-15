package qrcmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
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
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" QR-code "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)
			return cmdutil.Execute(cmd, args, m, handler.QR)
		},
	}

	cmdutil.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	cmdutil.FlagsFilter(c, cfg)

	c.AddCommand(newOpenCmd(cfg))

	return c
}

func newOpenCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"q"},
		Short:   "open QR-code image in default viewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Flags.Open = true
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" QR-code "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)
			return cmdutil.Execute(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
				for i := range bs {
					b := bs[i]
					qrcode := qr.New(b.URL)
					if err := qrcode.Generate(); err != nil {
						return err
					}
					if err := handler.OpenQR(cmd.Context(), qrcode, b, cfg.Name); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)
	cmdutil.HideFlag(c, "help")

	return c
}
