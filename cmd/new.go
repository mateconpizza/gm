package cmd

import (
	"github.com/spf13/cobra"
)

// newCmd represents the new command.
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "new bookmark, database, backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		return newRecordCmd.RunE(cmd, args)
	},
}

// newDatabaseCmd creates a new database.
var newDatabaseCmd = &cobra.Command{
	Use:     "database",
	Short:   "initialize a new bookmarks database",
	Aliases: []string{"db", "d"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return databaseNewCmd.RunE(cmd, args)
	},
}

// newBackupCmd creates a new backup.
var newBackupCmd = &cobra.Command{
	Use:     "backup",
	Short:   "create a new backup",
	Aliases: []string{"bk"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return backupNewCmd.RunE(cmd, args)
	},
}

// newBookmarkCmd creates a new bookmark.
var newBookmarkCmd = &cobra.Command{
	Use:     "record",
	Short:   "create a new record",
	Aliases: []string{"r"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return newRecordCmd.RunE(cmd, args)
	},
}

func init() {
	newCmd.AddCommand(newDatabaseCmd, newBackupCmd, newBookmarkCmd)
	rootCmd.AddCommand(newCmd)
}
