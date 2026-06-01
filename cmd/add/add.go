// Package add provides Cobra subcommands for creating new entities,
// including bookmarks, databases, and backups.
package add

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "add",
		Short:   "add a bookmark",
		Aliases: []string{"new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return handler.AddBookmark(cmd.Context(), d, args)
		},
	}

	c.Flags().SortFlags = false
	c.Flags().StringVar(&app.Flags.Title, "title", "", "bookmark title")
	c.Flags().StringVar(&app.Flags.TagsStr, "tags", "", "bookmark tags")

	return c
}
