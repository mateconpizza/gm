package io

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var htmlCmd = &cobra.Command{
	Use:   "html",
	Short: "Import from HTML Netscape file",
	RunE: func(_ *cobra.Command, _ []string) error {
		app := config.New()
		if app.Flags.Path == "" {
			return fmt.Errorf("%w: %q", ErrMissingArg, "filename")
		}

		file, err := os.Open(app.Flags.Path)
		if err != nil {
			log.Printf("Error opening file: %v, %q\n", err, app.Flags.Path)
			return err
		}
		defer func() {
			if err := file.Close(); err != nil {
				slog.Error("Err closing file", "file", app.Flags.Path)
			}
		}()

		if err := bookio.IsValidNetscapeFile(file); err != nil {
			return err
		}

		bp := bookio.NewHTMLParser()
		nbs, err := bp.ParseHTML(file)
		if err != nil {
			return err
		}

		bs := make([]*bookmark.Bookmark, 0, len(nbs))
		for i := range nbs {
			bs = append(bs, bookio.FromNetscape(&nbs[i]))
		}

		r, err := db.New(app.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		c := ui.NewConsole(
			ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
				r.Close()
				sys.ErrAndExit(err)
			}))),
		)

		s := color.Text("Found %d bookmarks from %q\n").Italic().String()
		c.F.Success(fmt.Sprintf(s, len(nbs), file.Name())).Flush()

		deduplicated := port.Deduplicate(c, r, bs)
		n := len(deduplicated)
		if n == 0 {
			return bookmark.ErrBookmarkNotFound
		}

		opt, err := c.Choose(fmt.Sprintf("Import %d bookmarks?", n), []string{"yes", "no", "select"}, "y")
		if err != nil {
			return err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			return sys.ErrActionAborted
		case "s", "select":
			m := menu.New[*bookmark.Bookmark](
				menu.WithInterruptFn(c.T.InterruptFn),
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s to import", false),
			)

			m.SetItems(deduplicated)
			m.SetPreprocessor(func(b **bookmark.Bookmark) string { return txt.Oneline(*b) })
			deduplicated, err = m.Select()
			if err != nil {
				return err
			}
			n = len(deduplicated)
		case "y", "yes":
			fmt.Println("importing items")
		}

		if err := r.InsertMany(context.Background(), deduplicated); err != nil {
			return err
		}
		fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d bookmarks", n)))

		return nil
	},
}
