package qrcmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var ErrInvalidFormat = errors.New("invalid format")

// Output image valid formats.
var validFormats = []string{"jpeg", "png", "jpg"}

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "qr [query]",
		Aliases: []string{"q"},
		Short:   "generate QR",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" QR-code "),
				menu.WithPreview(app.PreviewCmd("{1}")),
			)
			return cmdutil.Execute(cmd, args, m, handler.QR)
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	c.AddCommand(newOpenCmd(app), newGenQR(app))

	return c
}

func newOpenCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"q"},
		Short:   "open QR-code image in default viewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](app,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" QR-code "),
				menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				for i := range bs {
					b := bs[i]
					qrcode := qr.New(b.URL)
					if err := qrcode.Generate(); err != nil {
						return err
					}
					if err := handler.QROpen(cmd.Context(), qrcode, b, app.Name); err != nil {
						return err
					}
				}

				return nil
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.HideFlag(c, "help")

	return c
}

func newGenQR(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "text [string]",
		Short:   "generate QR image from text",
		Aliases: []string{"t"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			qrcode := qr.New(strings.Join(args, " "))
			if err := qrcode.Generate(); err != nil {
				return err
			}

			if app.Flags.Path != "" {
				ext := strings.ToLower(filepath.Ext(app.Flags.Path))
				switch ext {
				case ".png", ".jpg", ".jpeg":
					return qrcode.Write(app.Flags.Path)
				default:
					return fmt.Errorf("%w: %q (use: %s)", ErrInvalidFormat, ext, strings.Join(validFormats, "|"))
				}
			}

			fmt.Print(qrcode.String())

			return nil
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "output", "o", "",
		fmt.Sprintf("write QR image to file [%s]", strings.Join(validFormats, "|")))

	return c
}
