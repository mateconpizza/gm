package io

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdGit "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// imports bookmarks from various sources.
var (
	importFromDatabaseCmd = &cobra.Command{
		Use:     "database",
		Short:   "Import from database",
		Aliases: []string{"db"},
		RunE:    fromDatabaseFunc,
	}

	importFromBackupCmd = &cobra.Command{
		Use:     "backup",
		Short:   "Import from backup",
		Aliases: []string{"bk"},
		RunE:    fromBackupFunc,
	}

	gitCmd = &cobra.Command{
		Use:     "git",
		Short:   cmdGit.GitImportCmd.Short,
		Aliases: []string{"g"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdGit.CloneAndImport(cmd.Short, config.New())
		},
	}
)

func fromBackupFunc(command *cobra.Command, args []string) error {
	app := config.New()
	destRepo, err := db.New(app.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destRepo.Close()

	dbName := files.StripSuffixes(destRepo.Name())
	bks, err := files.List(app.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(bks) == 0 {
		return db.ErrBackupNotFound
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			destRepo.Close()
			sys.ErrAndExit(err)
		}))),
	)

	backupPath, err := handler.SelectBackupOne(c, bks)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcRepo, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcRepo.Close()

	c.T.SetInterruptFn(func(err error) {
		destRepo.Close()
		srcRepo.Close()
		sys.ErrAndExit(err)
	})

	if err := port.FromBackup(c, destRepo, srcRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func fromDatabaseFunc(command *cobra.Command, _ []string) error {
	app := config.New()
	rDest, err := db.New(app.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer rDest.Close()

	// FIX: refactor `SelectDatabase`, return a string (fullpath)
	srcDB, err := handler.SelectDatabase(rDest.Cfg.Fullpath())
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	rSrc, err := db.New(srcDB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer rSrc.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			rDest.Close()
			rSrc.Close()
			sys.ErrAndExit(err)
		}))),
	)

	if err := port.Database(c, rSrc, rDest); err != nil {
		return fmt.Errorf("import from database: %w", err)
	}

	return nil
}
