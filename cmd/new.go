package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

type newRecordType struct {
	title string
	tags  string
}

var newRecordF = &newRecordType{}

// newCmd represents the new command.
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "New bookmark, database, backup",
	Example: `  gm new db -n newDBName
  gm new --title='Some title' --tags='tag1 tag2'
  gm new bk`,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		return handler.CheckDBLocked(config.App.DBPath)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return newBookmarkCmd.RunE(cmd, args)
	},
}

// newDatabaseCmd creates a new database.
var newDatabaseCmd = &cobra.Command{
	Use:     "database",
	Short:   "Create a new bookmarks database",
	Aliases: []string{"db", "d"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return databaseNewCmd.RunE(cmd, args)
	},
}

// newBackupCmd creates a new backup.
var newBackupCmd = &cobra.Command{
	Use:     "backup",
	Short:   backupNewCmd.Short,
	Aliases: []string{"bk"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return backupNewCmd.RunE(cmd, args)
	},
}

// newBookmarkCmd creates a new bookmark.
var newBookmarkCmd = &cobra.Command{
	Use:     "record",
	Short:   "Create a new bookmark",
	Aliases: []string{"r"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := db.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		// setup terminal and interrupt func handler (ctrl+c,esc handler)
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		f := frame.New(frame.WithColorBorder(color.Gray))
		h := color.BrightYellow("Add Bookmark").String()
		f.Header(h + color.Gray(" (ctrl+c to exit)\n").Italic().String()).Rowln().Flush()

		b := bookmark.New()
		if err := handler.NewBookmark(f, t, r, b, newRecordF.title, newRecordF.tags, args); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := bookmark.Validate(b); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		return handler.SaveNewBookmark(t, f.Reset(), r, b)
	},
}

func init() {
	newBookmarkCmd.Flags().StringVarP(&newRecordF.title, "title", "t", "", "bookmark title")
	newBookmarkCmd.Flags().StringVarP(&newRecordF.tags, "tags", "T", "", "bookmark tags")
	newCmd.AddCommand(newBookmarkCmd)

	newDatabaseCmd.Flags().StringVarP(&DBName, "name", "n", "", "new database name")
	_ = newDatabaseCmd.MarkFlagRequired("name")
	newCmd.AddCommand(newDatabaseCmd, newBackupCmd)

	Root.AddCommand(newCmd)
}
