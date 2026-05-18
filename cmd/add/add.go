// Package add provides Cobra subcommands for creating new entities,
// including bookmarks, databases, and backups.
package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/files"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "add a bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			c, p := d.Console(), d.Console().Palette()
			r, err := d.Repository()
			if err != nil {
				return err
			}
			title := p.BrightYellow.With(p.Bold).
				Sprint("Add Bookmark")

			comment := p.Dim.With(p.Italic).
				Sprint(" (ctrl-c to exit)")

			name := p.BrightYellow.With(p.Bold).
				Sprint(files.StripSuffixes(r.Name()))

			info := p.Gray.With(p.Italic).
				Sprintf(" (%d bookmarks)", r.Count(d.Context(), "bookmarks"))

			subtitle := p.Gray.With(p.Italic).
				Sprint("repo: " + name)

			c.Frame().
				Headerln(title + comment).
				Headerln(subtitle + info).
				Rowln().Flush()

			b := bookmark.New()
			if err := handler.NewBookmark(d, b, args); err != nil {
				return err
			}
			if err := bookmark.Validate(b); err != nil {
				return err
			}
			if err := handler.SaveNewBookmark(d, b); err != nil {
				return err
			}
			if err := git.AddBookmark(app, b); err != nil {
				return err
			}
			fmt.Println(c.SuccessMesg("bookmark added"))
			return nil
		},
	}
	c.Flags().SortFlags = false
	c.Flags().StringVar(&app.Flags.Title, "title", "", "bookmark title")
	c.Flags().StringVar(&app.Flags.TagsStr, "tags", "", "bookmark tags")
	return c
}
