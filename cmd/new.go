package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
)

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
	rootCmd.AddCommand(newCmd)
}
