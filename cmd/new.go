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
	"github.com/mateconpizza/gm/internal/ui/txt"
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

		cgi := func(s string) string { return color.BrightGray(s).Italic().String() }
		cy := func(s string) string { return color.BrightYellow(s).String() }
		f := frame.New(frame.WithColorBorder(color.Gray))
		f.Headerln(cy("Add Bookmark" + cgi(" (ctrl+c to exit)"))).Rowln().Flush()

		b := bookmark.New()
		if err := handler.NewBookmark(f, t, r, b, newRecordF.title, newRecordF.tags, args); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := bookmark.Validate(b); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if err := handler.SaveNewBookmark(t, f.Reset(), r, b); err != nil {
			return err
		}

		fmt.Print(txt.SuccessMesg("bookmark added\n"))

		return nil
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
