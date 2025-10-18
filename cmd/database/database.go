// Package database handles bookmarks database management operations.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	// dbRootCmd database management.
	dbRootCmd = &cobra.Command{
		Use:     "db",
		Aliases: []string{"database", "d"},
		Short:   "Database management",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.New()
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
			case a.Cfg.Flags.JSON:
				return printer.RepoInfo(a)

			case a.Cfg.Flags.Vacuum:
				slog.Debug("database:", "vacuum", a.Cfg.DBName)
				r, err := db.New(a.Cfg.DBPath)
				if err != nil {
					return fmt.Errorf("backup: %w", err)
				}
				defer r.Close()

				return r.Vacuum(cmd.Context())

			case a.Cfg.Flags.Reorder:
				slog.Debug("database: reordering bookmark IDs")
				r, err := db.New(a.Cfg.DBPath)
				if err != nil {
					return fmt.Errorf("backup: %w", err)
				}
				defer r.Close()

				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()

				return r.ReorderIDs(ctx)

			case a.Cfg.Flags.Lock:
				return lockCmd.RunE(cmd, args)
			case a.Cfg.Flags.Unlock:
				return unlockCmd.RunE(cmd, args)
			case a.Cfg.Flags.List:
				return printer.DatabasesTable(cmd.Context(), a.Cfg.Path.Data)
			case a.Cfg.Flags.Info:
				return infoCmd.RunE(cmd, args)
			}

			return cmd.Usage()
		},
	}

	// newCmd initialize a new bookmarks database.
	newCmd = &cobra.Command{
		Use:               "new",
		Short:             "Initialize a new bookmarks database",
		Example:           `  gm db new -n newDBName`,
		Annotations:       cli.SkipDBCheckAnnotation,
		PersistentPreRunE: setup.InitCmd.PersistentPreRunE,
		RunE:              setup.InitCmd.RunE,
		PostRunE:          setup.InitCmd.PostRunE,
	}

	// dropCmd drops a database.
	dropCmd = &cobra.Command{
		Use:      "drop",
		Short:    "Drop a database",
		RunE:     dbDropFunc,
		PostRunE: dbDropPostFunc,
	}

	// infoCmd shows information about a database.
	infoCmd = &cobra.Command{
		Use:     "info",
		Short:   "Show information about a database",
		Aliases: []string{"i", "show"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg := config.New()
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
		},
	}

	// rmCmd remove a database.
	rmCmd = &cobra.Command{
		Use:      "rm",
		Short:    dbRemoveCmd.Short,
		Aliases:  []string{"r", "remove"},
		Example:  dbRemoveCmd.Example,
		RunE:     dbRemoveCmd.RunE,
		PostRunE: dbRemoveCmd.PostRunE,
	}

	lockCmd = &cobra.Command{
		Use:   "lock",
		Short: "Lock a database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := ui.NewConsole(
				ui.WithTerminal(
					terminal.New(
						terminal.WithContext(cmd.Context()),
						terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
					),
				),
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
			)

			cfg := config.New()

			return handler.LockRepo(c, cfg.DBPath)
		},
	}

	unlockCmd = &cobra.Command{
		Use:         "unlock",
		Short:       "Unlock a database",
		Annotations: cli.SkipDBCheckAnnotation,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := ui.NewConsole(
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Purple))),
				ui.WithTerminal(
					terminal.New(
						terminal.WithContext(cmd.Context()),
						terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) })),
				),
			)

			cfg := config.New()

			return handler.UnlockRepo(c, cfg.DBPath)
		},
	}
)

func dbDropFunc(cmd *cobra.Command, _ []string) error {
	cfg := config.New()
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
	cfg := config.New()
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

	fmt.Print(c.SuccessMesg("database dropped\n"))

	if !c.Confirm("Untrack database?", "n") {
		return nil
	}

	if err := gr.Untrack("untracked"); err != nil {
		return err
	}

	fmt.Print(c.SuccessMesg("database untracked\n"))

	return nil
}

func NewCmd() *cobra.Command {
	cfg := config.New()
	f := dbRootCmd.Flags()
	f.SortFlags = false

	f.StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "database name")
	f.BoolVarP(&cfg.Flags.List, "list", "l", false, "list databases")
	f.BoolVarP(&cfg.Flags.Info, "info", "i", false, "database information")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&cfg.Flags.Vacuum, "vacuum", "X", false, "rebuilds the database file")
	f.BoolVarP(&cfg.Flags.Reorder, "reorder", "R", false, "reorder IDs")
	f.BoolVarP(&cfg.Flags.Lock, "lock", "L", false, "lock a database")
	f.BoolVarP(&cfg.Flags.Unlock, "unlock", "U", false, "unlock a database")

	// new database
	newCmd.Flags().StringVarP(&cfg.DBName, "name", "n", config.MainDBName,
		"new database name")
	dbRootCmd.AddCommand(newCmd)
	// show database info
	infoCmd.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false,
		"output in JSON format")
	// remove database
	rmCmd.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false,
		"select database to remove (fzf compatible)")

	// backup
	backupUnlockCmd.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false,
		"select a backup to lock|unlock (fzf compatible)")
	backupCmd.AddCommand(BackupNewCmd, backupRmCmd, backupLockCmd, backupUnlockCmd)
	dbRootCmd.AddCommand(backupCmd)

	// remove
	removeCmd.AddCommand(dbRemoveCmd, bkRemoveCmd)
	dbRootCmd.AddCommand(removeCmd)

	dbRootCmd.AddCommand(dropCmd, rmCmd)

	return dbRootCmd
}
