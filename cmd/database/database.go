// Package database handles bookmarks database management operations.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	// dbRootCmd database management.
	dbRootCmd = &cobra.Command{
		Use:     "db",
		Aliases: []string{"database", "d"},
		Short:   "database ops",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			if cfg.Flags.Unlock {
				return unlockCmd.RunE(cmd, args)
			}

			r, err := db.New(cfg.DBPath)
			if err != nil {
				return err
			}

			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithDB(r),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
					db.Shutdown()
				})),
			)

			switch {
			case a.Cfg.Flags.Vacuum:
				slog.Debug("database:", "vacuum", a.Cfg.DBName)
				defer r.Close()
				return r.Vacuum(cmd.Context())

			case a.Cfg.Flags.Reorder:
				slog.Debug("database: reordering bookmark IDs")
				defer r.Close()

				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()

				return r.ReorderIDs(ctx)

			case a.Cfg.Flags.Lock:
				return lockCmd.RunE(cmd, args)
			}

			return cmd.Usage()
		},
	}

	// createCmd initialize a new bookmarks database.
	createCmd = &cobra.Command{
		Use:               "create",
		Short:             "create a database",
		Aliases:           []string{"add"},
		Example:           `  gm db create -n myDb`,
		Annotations:       cli.SkipDBCheckAnnotation,
		PersistentPreRunE: setup.InitCmd.PersistentPreRunE,
		RunE:              setup.InitCmd.RunE,
		PostRunE:          setup.InitCmd.PostRunE,
	}

	listCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls"},
		Short:   "list all databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}
			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
					db.Shutdown()
				})),
			)

			return printer.DatabasesTable(cmd.Context(), a.Console(), a.Cfg.Path.Data)
		},
	}

	// dropCmd drops a database.
	dropCmd = &cobra.Command{
		Use:      "drop",
		Short:    "drop a database",
		RunE:     dbDropFunc,
		PostRunE: dbDropPostFunc,
	}

	// infoCmd shows information about a database.
	infoCmd = &cobra.Command{
		Use:     "info",
		Short:   "show information about a database",
		Aliases: []string{"i", "show"},
		RunE:    dbInfoFunc,
	}

	lockCmd = &cobra.Command{
		Use:   "lock",
		Short: "lock a database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
			)

			return handler.LockRepo(a, cfg.DBPath)
		},
	}

	unlockCmd = &cobra.Command{
		Use:         "unlock",
		Short:       "unlock a database",
		Annotations: cli.SkipDBCheckAnnotation,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
			)

			return handler.UnlockRepo(a, cfg.DBPath)
		},
	}
)

func dbDropFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer r.Close()

	a := app.New(cmd.Context(),
		app.WithDB(r),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	return handler.DroppingDB(a)
}

func dbDropPostFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if !cfg.Git.Enabled {
		return nil
	}

	gr, err := git.NewRepo(cfg.DBPath)
	if err != nil {
		return err
	}
	if !gr.IsTracked() || !files.Exists(gr.Loc.DBPath) {
		return nil
	}

	c := ui.NewConsole(ui.WithDefaultTerminal(cmd.Context(), func(err error) { sys.ErrAndExit(err) }))

	if err := gr.Drop("dropped"); err != nil {
		return err
	}

	fmt.Println(c.SuccessMesg("database dropped"))

	if !c.Confirm("Untrack database?", "n") {
		return nil
	}

	if err := gr.Untrack("untracked"); err != nil {
		return err
	}

	fmt.Println(c.SuccessMesg("database untracked"))

	return nil
}

func dbInfoFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}

	a := app.New(cmd.Context(),
		app.WithConfig(cfg),
		app.WithDB(r),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			db.Shutdown()
		})),
	)

	return printer.RepoInfo(a)
}

func NewCmd(cfg *config.Config) *cobra.Command {
	f := dbRootCmd.Flags()
	f.SortFlags = false

	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&cfg.Flags.Vacuum, "vacuum", "X", false, "rebuilds the database file")
	f.BoolVarP(&cfg.Flags.Reorder, "reorder", "R", false, "reorder IDs")
	f.BoolVarP(&cfg.Flags.Lock, "lock", "L", false, "lock a database")
	f.BoolVarP(&cfg.Flags.Unlock, "unlock", "U", false, "unlock a database")

	dbRootCmd.AddCommand(
		createCmd,
		listCmd,
		infoCmd,
		dbRemoveCmd,
		backupCmd,
		dropCmd,
	)

	// new database
	createCmd.Flags().StringVar(&cfg.DBName, "db", config.MainDBName, "new database name")

	// show database info
	infoCmd.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false,
		"output in JSON format")

	// backup
	cmdutil.FlagMenu(backupLockCmd, cfg)
	backupCmd.AddCommand(
		BackupNewCmd,
		backupRmCmd,
		backupLockCmd,
		backupUnlockCmd,
	)

	return dbRootCmd
}
