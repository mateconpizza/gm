package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/handler"
)

var ErrImportSourceNotFound = errors.New("import source not found")

// importFromCmd imports bookmarks from various sources.
var importFromCmd = &cobra.Command{
	Use:   "import",
	Short: "Import bookmarks from various sources",
	RunE:  handler.SelectSource,
}

var importFromDatabaseCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db"},
	Short:   "Import bookmarks from database",
	RunE:    handler.ImportFromDatabase,
}

var importFromBackupCmd = &cobra.Command{
	Use:     "backup",
	Short:   "Import bookmarks from backup",
	Aliases: []string{"bk"},
	RunE:    handler.ImportFromBackup,
}

var importFromBrowserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Import bookmarks from browser",
	RunE:  handler.ImportFromBrowser,
}

func init() {
	importFromCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	importFromDatabaseCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	importFromCmd.AddCommand(importFromBackupCmd, importFromBrowserCmd, importFromDatabaseCmd)
	rootCmd.AddCommand(importFromCmd)
}
