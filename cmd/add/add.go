// Package add provides Cobra subcommands for creating new entities,
// including bookmarks, databases, and backups.
package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "add bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			cfg.Flags.Create = true

			r, err := db.New(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			defer r.Close()

			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithDB(r),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
					db.Shutdown()
					sys.ErrAndExit(err)
				})),
			)

			c, p := a.Console(), a.Console().Palette()
			s := p.BrightYellow.Sprint("Add Bookmark") + p.Dim.With(p.Italic).Sprint(" (ctrl+c to exit)")
			c.Frame().Headerln(s).Rowln().Flush()

			b := bookmark.New()
			if err := handler.NewBookmark(a, b, args); err != nil {
				return err
			}

			if err := bookmark.Validate(b); err != nil {
				return err
			}

			if err := handler.SaveNewBookmark(a, b); err != nil {
				return err
			}

			if err := git.AddBookmark(cfg, b); err != nil {
				return err
			}

			fmt.Println(c.SuccessMesg("bookmark added"))

			return nil
		},
	}
	c.Flags().StringVar(&cfg.Flags.Title, "title", "", "bookmark title")
	c.Flags().StringVarP(&cfg.Flags.TagsStr, "tags", "t", "", "bookmark tags")
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	return c
}
