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
	c := &cobra.Command{
		Use:                "io",
		Short:              "export/import bookmarks",
		RunE:               cli.HookHelp,
		PersistentPostRunE: cli.HookGitSync,
	}

	c.Flags().BoolVarP(&cfg.Flags.Help, "help", "h", false, "")
	_ = c.Flags().MarkHidden("help")

	c.AddCommand(
		newExportCmd(cfg),
		browserCmd,
		newHTMLCmd(cfg),
		importFromBackupCmd,
		importFromDatabaseCmd,
	)

	return c
}
