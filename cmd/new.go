package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

type newRecordType struct {
	title string
	tags  string
}

var newRecordFlags = &newRecordType{}

var titleFlag string

// newCmd represents the new command.
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "New bookmark, database, backup",
	Example: `  gm new db -n newDBName
  gm new bk`,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		return handler.CheckDBLocked(config.App.DBPath)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return newRecordCmd.RunE(cmd, args)
	},
}

var newRecordCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new bookmark",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(config.App.DBPath)
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
		f.Header(h + color.Gray(" (ctrl+c to exit)\n").Italic().String()).Row("\n")

		b := bookmark.New()
		if err := handler.NewBookmark(f.Flush(), t, r, b, newRecordFlags.title, newRecordFlags.tags, args); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := bookmark.Validate(b); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if err := handler.SaveNewBookmark(t, f, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := r.InsertOne(context.Background(), b); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handler.GitCommit("Add"); err != nil {
			return fmt.Errorf("%w", err)
		}

		success := color.BrightGreen("Successfully").Italic().String()
		f.Clear().Success(success + " bookmark created\n").Flush()

		return nil
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
	Short:   newRecordCmd.Short,
	Aliases: []string{"r"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return newRecordCmd.RunE(cmd, args)
	},
}

func init() {
	newBookmarkCmd.Flags().StringVar(&titleFlag, "title", "", "new bookmark title")
	newDatabaseCmd.Flags().StringVarP(&DBName, "name", "n", "", "new database name")
	_ = newDatabaseCmd.MarkFlagRequired("name")
	newCmd.AddCommand(newDatabaseCmd, newBackupCmd, newBookmarkCmd)

	newRecordCmd.Flags().StringVarP(&newRecordFlags.title, "title", "t", "", "bookmark title")
	newRecordCmd.Flags().StringVarP(&newRecordFlags.tags, "tags", "T", "", "bookmark tags")
	newCmd.AddCommand(newRecordCmd)

	rootCmd.AddCommand(newCmd)
}
