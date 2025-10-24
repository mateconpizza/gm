package io

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdGit "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
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
		Short:   cmdGit.ImportCmd.Short,
		Aliases: []string{"g"},
		RunE:    cmdGit.ImportCmd.RunE,
	}
)

func fromBackupFunc(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	destRepo, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destRepo.Close()

	dbName := files.StripSuffixes(destRepo.Name())
	bks, err := files.List(cfg.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(bks) == 0 {
		return db.ErrBackupNotFound
	}

	a := app.New(cmd.Context(),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			db.Shutdown()
			sys.ErrAndExit(err)
		})),
	)

	backupPath, err := handler.SelectBackupOne(a, bks)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcRepo, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcRepo.Close()

	if err := port.FromBackup(a, destRepo, srcRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func fromDatabaseFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	rDest, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer rDest.Close()

	a := app.New(cmd.Context(),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			db.Shutdown()
			sys.ErrAndExit(err)
		})),
	)

	// FIX: refactor `SelectDatabase`, return a string (fullpath)
	srcDB, err := handler.SelectDatabase(a, rDest.Cfg.Fullpath())
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	rSrc, err := db.New(srcDB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer rSrc.Close()

	if err := port.Database(a, rSrc, rDest); err != nil {
		return fmt.Errorf("import from database: %w", err)
	}

	return nil
}
