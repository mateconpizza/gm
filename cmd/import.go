package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/importer"
	"github.com/mateconpizza/gm/internal/repo"
)

var ErrImportSourceNotFound = errors.New("import source not found")

// importFromCmd imports bookmarks from various sources.
var importFromCmd = &cobra.Command{
	Use:   "import",
	Short: "Import bookmarks from various sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var importFromDatabaseCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db"},
	Short:   "Import bookmarks from database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return handler.ImportFromDatabase()
	},
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
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		if err := importer.Browser(r); err != nil {
			return fmt.Errorf("import from browser: %w", err)
		}

		if err := handler.GitCommit("Import from Browser"); err != nil {
			if errors.Is(err, git.ErrGitNothingToCommit) {
				return nil
			}
			return fmt.Errorf("commit: %w", err)
		}

		return nil
	},
}

var importFromGitRepoCmd = &cobra.Command{
	Use:   "git",
	Short: "Import bookmarks from git repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		return gitCloneCmd.RunE(cmd, args)
	},
}

func init() {
	importFromCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	importFromDatabaseCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	importFromCmd.AddCommand(
		importFromBackupCmd,
		importFromBrowserCmd,
		importFromDatabaseCmd,
		importFromGitRepoCmd,
	)
	rootCmd.AddCommand(importFromCmd)
}
