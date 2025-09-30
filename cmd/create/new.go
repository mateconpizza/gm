// Package create provides Cobra subcommands for creating new entities,
// including bookmarks, databases, and backups.
package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/database"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
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
		RunE:              newBookmarkCmd.RunE,
		PersistentPreRunE: cli.HookEnsureDatabase,
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
func newBookmarkFunc(command *cobra.Command, args []string) error {
	app := config.New()
	r, err := db.New(app.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	cgi := func(s string) string { return color.BrightGray(s).Italic().String() }
	cy := func(s string) string { return color.BrightYellow(s).String() }
	c.F.Headerln(cy("Add Bookmark" + cgi(" (ctrl+c to exit)"))).Rowln().Flush()

	b := bookmark.New()
	if err := handler.NewBookmark(c, r, b, app.Flags.Title, app.Flags.TagsStr, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := handler.SaveNewBookmark(c, r, b, app); err != nil {
		return err
	}

	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}

	if gr.IsTracked() {
		if err := gr.Add([]*bookmark.Bookmark{b}); err != nil {
			return err
		}
		if err := gr.RepoStatsWrite(); err != nil {
			return err
		}
		if err := gr.Commit("new bookmark"); err != nil {
			return err
		}
	}

	fmt.Print(c.SuccessMesg("bookmark added\n"))

	return nil
}

func NewCmd() *cobra.Command {
	app := config.New()
	newBookmarkCmd.Flags().StringVarP(&app.Flags.Title, "title", "t", "", "bookmark title")
	newBookmarkCmd.Flags().StringVarP(&app.Flags.TagsStr, "tags", "T", "", "bookmark tags")
	newDatabaseCmd.Flags().StringVarP(&app.DBName, "name", "n", config.MainDBName,
		"database name")
	newCmd.AddCommand(newBookmarkCmd, newDatabaseCmd, newBackupCmd)
	_ = newDatabaseCmd.Flags().MarkHidden("help")

	return newCmd
}
