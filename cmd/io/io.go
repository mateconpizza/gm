// Package io contains functions to export/import bookmarks.
package io

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
)

var (
	ErrImportSourceNotFound = errors.New("import source not found")
	ErrMissingArg           = errors.New("missing argument")
)

func NewCmd(cfg *config.Config) *cobra.Command {
	ioCmd := &cobra.Command{
		Use:                "io",
		Short:              "Export/Import bookmarks",
		RunE:               cli.HookHelp,
		PersistentPostRunE: cli.HookGitSync,
	}

	ioCmd.Flags().BoolVarP(&cfg.Flags.Export, "export", "e", false, "export selected bookmarks")
	ioCmd.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "menu mode (fzf)")

	// browser
	ioCmd.AddCommand(browserCmd)

	// netscape html
	htmlCmd.Flags().StringVarP(&cfg.Flags.Path, "filename", "f", "", "filename path")
	ioCmd.AddCommand(htmlCmd)

	// databases
	ioCmd.AddCommand(importFromBackupCmd)
	ioCmd.AddCommand(importFromDatabaseCmd)

	// git
	gitCmd.Flags().StringVarP(&cfg.Flags.Path, "uri", "i", "", "repo URI to import")
	ioCmd.AddCommand(gitCmd)

	return ioCmd
}
