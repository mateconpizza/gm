// Package io contains functions to export/import bookmarks.
package io

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrImportSourceNotFound = errors.New("import source not found")
	ErrMissingArg           = errors.New("missing argument")
)

func NewCmd() *cobra.Command {
	app := config.New()

	ioCmd.Flags().BoolVarP(&app.Flags.Export, "export", "e", false, "export selected bookmarks")
	ioCmd.Flags().BoolVarP(&app.Flags.Menu, "menu", "m", false, "menu mode (fzf)")

	// browser
	ioCmd.AddCommand(browserCmd)

	// netscape html
	htmlCmd.Flags().StringVarP(&app.Flags.Path, "filename", "f", "", "filename path")
	ioCmd.AddCommand(htmlCmd)

	// databases
	ioCmd.AddCommand(importFromBackupCmd)
	ioCmd.AddCommand(importFromDatabaseCmd)

	// git
	gitCmd.Flags().StringVarP(&app.Flags.Path, "uri", "i", "", "repo URI to import")
	ioCmd.AddCommand(gitCmd)

	return ioCmd
}

var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "Export/Import bookmarks",
	RunE:  cli.HookHelp,
}

func menuSelect[T bookmark.Bookmark]() *menu.Menu[T] {
	app := config.New()
	mo := []menu.OptFn{
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(app.Cmd + " --name " + app.DBName + " records {1}"),
		menu.WithHeader("Select records for import/export", false),
	}

	if app.Flags.Multiline {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}
