// Package io contains functions to export/import bookmarks.
package io

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

var (
	ErrImportSourceNotFound = errors.New("import source not found")
	ErrMissingArg           = errors.New("missing argument")
)

var ioFlags *config.Flags

func init() {
	ioFlags = config.NewFlags()

	ioCmd.Flags().BoolVarP(&ioFlags.Export, "export", "e", false, "export selected bookmarks")
	ioCmd.Flags().BoolVarP(&ioFlags.Menu, "menu", "m", false, "menu mode (fzf)")

	// browser
	ioCmd.AddCommand(browserCmd)

	// netscape html
	htmlCmd.Flags().StringVarP(&ioFlags.Path, "filename", "f", "", "filename path")
	ioCmd.AddCommand(htmlCmd)

	// databases
	ioCmd.AddCommand(importFromBackupCmd)
	ioCmd.AddCommand(importFromDatabaseCmd)

	// git
	gitCmd.Flags().StringVarP(&ioFlags.Path, "uri", "i", "", "repo URI to import")
	ioCmd.AddCommand(gitCmd)

	cmd.Root.AddCommand(ioCmd)
}

var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "Export/Import bookmarks",
	RunE: func(c *cobra.Command, args []string) error {
		cfg := config.App
		r, err := db.New(cfg.DBPath)
		if err != nil {
			return err
		}
		defer r.Close()

		bs, err := handler.Data(menuSelect(), r, args, ioFlags)
		if err != nil {
			return err
		}

		fmt.Printf("len(bs): %v\n", len(bs))

		return c.Help()
	},
}

func menuSelect[T bookmark.Bookmark]() *menu.Menu[T] {
	mo := []menu.OptFn{
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithPreview(config.App.Cmd + " --name " + config.App.DBName + " records {1}"),
		menu.WithHeader("Select records for import/export", false),
	}

	if ioFlags.Multiline {
		mo = append(mo, menu.WithMultilineView())
	}

	return menu.New[T](mo...)
}
