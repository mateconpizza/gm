// Package create provides Cobra subcommands for creating new entities,
// including bookmarks, databases, and backups.
package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/database"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var (
	// newCmd represents the new command.
	newCmd = &cobra.Command{
		Use:   "new",
		Short: "New bookmark, database, backup",
		Example: `  gm new db -n newDBName
  gm new r --title='Some title' --tags='tag1 tag2'
  gm new bk`,
		RunE: newBookmarkCmd.RunE,
	}

	// newDatabaseCmd creates a new database.
	newDatabaseCmd = &cobra.Command{
		Use:               "database",
		Short:             "Create a new bookmarks database",
		Aliases:           []string{"db", "d"},
		Annotations:       setup.InitCmd.Annotations,
		PersistentPreRunE: setup.InitCmd.PersistentPreRunE,
		RunE:              setup.InitCmd.RunE,
		PostRunE:          setup.InitCmd.PostRunE,
	}

	// newBackupCmd creates a new backup.
	newBackupCmd = &cobra.Command{
		Use:     "backup",
		Short:   database.BackupNewCmd.Short,
		Aliases: []string{"bk"},
		RunE:    database.BackupNewCmd.RunE,
	}

	newBookmarkCmd = &cobra.Command{
		Use:     "record",
		Short:   "Create a new bookmark",
		Aliases: []string{"r"},
		RunE:    newBookmarkFunc,
	}
)

// newBookmarkCmd creates a new bookmark.
func newBookmarkFunc(cmd *cobra.Command, args []string) error {
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
	s := p.BrightYellow.Sprint("Add Bookmark") + p.BrightBlack.With(p.Italic).Sprint(" (ctrl+c to exit)")
	c.Frame().Headerln(s).Rowln().Flush()

	b := bookmark.New()
	if err := handler.NewBookmark(a, b, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := handler.SaveNewBookmark(a, b); err != nil {
		return err
	}

	if err := git.AddBookmark(cfg, b); err != nil {
		return err
	}

	fmt.Println(c.SuccessMesg("bookmark added"))

	return nil
}

func NewCmd(cfg *config.Config) *cobra.Command {
	newBookmarkCmd.Flags().StringVarP(&cfg.Flags.Title, "title", "t", "", "bookmark title")
	newBookmarkCmd.Flags().StringVarP(&cfg.Flags.TagsStr, "tags", "T", "", "bookmark tags")
	newDatabaseCmd.Flags().StringVarP(&cfg.DBName, "name", "n", config.MainDBName,
		"database name")
	newCmd.AddCommand(newBookmarkCmd, newDatabaseCmd, newBackupCmd)
	_ = newDatabaseCmd.Flags().MarkHidden("help")

	return newCmd
}
