package archive

import (
	"fmt"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "archive [query]",
		Aliases: []string{"snap", "ar", "a"},
		Short:   "show archive URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](
				cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" archive URL "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(a *app.Context, bs []*bookmark.Bookmark) error {
				n := len(bs)
				if n == 0 {
					return handler.ErrNoItems
				}

				var sb strings.Builder
				for _, u := range bs {
					sb.WriteString(u.ArchiveURL + "\n")
				}

				fmt.Print(sb.String())

				return nil
			}, onlySnapshots)
		},
	}

	cmdutil.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	cmdutil.FlagsFilter(c, cfg)

	c.AddCommand(newLookupCmd(cfg), newOpenCmd(cfg))

	return c
}

func newOpenCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open archive URL in browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](
				cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" open archive URL "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
				menu.WithNth("3.."),
			)
			m.SetFormatter(formatArchiveURL)

			return cmdutil.Execute(cmd, args, m, handler.Open, onlySnapshots)
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	return c
}

func formatArchiveURL(b *bookmark.Bookmark) string {
	absolute, relative := txt.TimeWithAgo(b.ArchiveTimestamp)
	idStr := ansi.Dim.Sprintf("%d", b.ID)
	title := ansi.Normal.Sprint(b.Title)
	if b.Title == "" {
		title = ansi.Dim.Sprint(b.URL)
	}

	title = runewidth.Truncate(title, terminal.MaxWidth, "…")
	relative = ansi.BrightBlack.Wrap("("+relative+")", ansi.Italic)

	return fmt.Sprintf("%s %s %s %-*s %s",
		idStr,
		txt.UnicodeDash,
		absolute,
		28,
		relative,
		title,
	)
}

func onlySnapshots(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	filtered := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		if bs[i].ArchiveURL != "" {
			filtered = append(filtered, bs[i])
		}
	}

	if len(filtered) == 0 {
		return filtered
	}

	result := make([]*bookmark.Bookmark, 0, len(filtered))
	for i := range filtered {
		f := filtered[i]
		b := bookmark.New()
		// TODO: use `f.Title` or keep `f.URL`?
		b.Title = f.URL
		b.ID = f.ID
		b.URL = f.ArchiveURL
		b.ArchiveTimestamp = f.ArchiveTimestamp
		b.ArchiveURL = b.URL
		result = append(result, b)
	}

	return result
}
