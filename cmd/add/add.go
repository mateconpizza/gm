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
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "add bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			c, p := d.Console(), d.Console().Palette()
			s := p.BrightYellow.Sprint("Add Bookmark") + p.Dim.With(p.Italic).Sprint(" (ctrl+c to exit)")
			c.Frame().Headerln(s).Rowln().Flush()

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
	c.Flags().StringVar(&app.Flags.Title, "title", "", "bookmark title")
	c.Flags().StringVarP(&app.Flags.TagsStr, "tags", "t", "", "bookmark tags")
	cmdutil.HideFlag(c, "help")

	return c
}
