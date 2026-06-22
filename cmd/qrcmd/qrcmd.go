package qrcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/bookmark/qr"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "qr [query]",
		Aliases: []string{"q"},
		Short:   "generate QR",
		Example: app.Example(`  $ {cmd} qr golang
  $ {cmd} qr --menu
  $ {cmd} qr open "github"
  $ {cmd} qr text "https://golang.org" --output go.png
  $ {cmd} qr text "https://golang.org"`),
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
		Example: app.Example(`  $ {cmd} qr open "golang"
  $ {cmd} qr open --menu
  $ {cmd} qr open --head 10
  $ {cmd} qr open --tag devops
  $ {cmd} qr open "api" --force`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(cmd, args, setupMenu(app), handler.QROpen)
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
		Example: app.Example(`  $ {cmd} qr text "https://golang.org"
  $ {cmd} qr text "otpauth://totp/example"
  $ {cmd} qr text "https://golang.org" --output go.png
  $ {cmd} qr text "hello world" --output hello.jpg`),
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
				return handler.QRSave(qrcode, app.Flags.Path)
			}

			fmt.Fprint(os.Stdout, qrcode.String())

			return nil
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "output", "o", "",
		fmt.Sprintf("output image path (%s)", strings.Join(handler.QRFormats, ", ")))

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
