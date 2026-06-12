package qrcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
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
			return cmdutil.Execute(cmd, args, setupMenu(app), handler.QR)
		},
	}
	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	c.AddCommand(newOpenCmd(app), newGenQR(app))

	return c
}

func newOpenCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"q"},
		Short:   "open QR-code image in default viewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
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
			}

			return cmdutil.Execute(cmd, args, setupMenu(app), a)
		},
	}

	cmdutil.FlagSort(c, app, handler.SortSupported)
	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

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

			text := strings.Join(args, " ")
			qrcode := qr.New(text)
			if err := qrcode.Generate(); err != nil {
				return err
			}

			if app.Flags.Path != "" {
				return outputQR(qrcode, app.Flags.Path)
			}

			fmt.Fprint(os.Stdout, qrcode.String())

			return nil
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "output", "o", "",
		"write QR image to file: "+strings.Join(validFormats, ", "))

	return c
}

func setupMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	return picker.New[bookmark.Bookmark](
		app,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" QR-code "),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), "{1}")),
	)
}

func outputQR(qrcode *qr.QRCode, s string) error {
	ext := strings.ToLower(filepath.Ext(s))
	switch ext {
	case ".png", ".jpg", ".jpeg":
		fn, err := files.NormalizePath(s, "qr.png")
		if err != nil {
			return err
		}
		return qrcode.Write(fn)
	default:
		return fmt.Errorf("%w: %q (use: %s)", ErrInvalidFormat, ext, strings.Join(validFormats, "|"))
	}
}
