// Package database handles bookmarks database management operations.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/setup"
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
			app := config.New()

			switch {
			case app.Flags.JSON:
				c := ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))))
				return printer.RepoInfo(c, app)

			case app.Flags.Vacuum:
				slog.Debug("database:", "vacuum", app.DBName)
				r, err := db.New(app.DBPath)
				if err != nil {
					return fmt.Errorf("backup: %w", err)
				}
				defer r.Close()

				return r.Vacuum(context.Background())

			case app.Flags.Reorder:
				slog.Debug("database: reordering bookmark IDs")
				r, err := db.New(app.DBPath)
				if err != nil {
					return fmt.Errorf("backup: %w", err)
				}
				defer r.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				return r.ReorderIDs(ctx)

			case app.Flags.Lock:
				return lockCmd.RunE(cmd, args)
			case app.Flags.Unlock:
				return unlockCmd.RunE(cmd, args)
			case app.Flags.List:
				return printer.DatabasesTable(app.Path.Data)
			case app.Flags.Info:
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
		RunE: func(_ *cobra.Command, _ []string) error {
			app := config.New()
			return printer.RepoInfo(
				ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray)))),
				app,
			)
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
		RunE: func(_ *cobra.Command, _ []string) error {
			c := ui.NewConsole(
				ui.WithTerminal(
					terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) })),
				),
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
			)

			app := config.New()

			return handler.LockRepo(c, app.DBPath)
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
					terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) })),
				),
			)

			app := config.New()

			return handler.UnlockRepo(c, app.DBPath)
		},
	}
)

func dbDropFunc(_ *cobra.Command, _ []string) error {
	app := config.New()
	r, err := db.New(app.DBPath)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	return handler.DroppingDB(c, r, app.Path.Backup, app.Flags.Force)
}

func dbDropPostFunc(_ *cobra.Command, _ []string) error {
	app := config.New()
	if !git.IsInitialized(app.Git.Path) {
		return nil
	}

	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}
	if !gr.IsTracked() || !files.Exists(gr.Loc.DBPath) {
		return nil
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
	)

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
	app := config.New()
	f := dbRootCmd.Flags()
	f.SortFlags = false

	f.StringVarP(&app.DBName, "name", "n", config.MainDBName, "database name")
	f.BoolVarP(&app.Flags.List, "list", "l", false, "list databases")
	f.BoolVarP(&app.Flags.Info, "info", "i", false, "database information")
	f.BoolVarP(&app.Flags.JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&app.Flags.Vacuum, "vacuum", "X", false, "rebuilds the database file")
	f.BoolVarP(&app.Flags.Reorder, "reorder", "R", false, "reorder IDs")
	f.BoolVarP(&app.Flags.Lock, "lock", "L", false, "lock a database")
	f.BoolVarP(&app.Flags.Unlock, "unlock", "U", false, "unlock a database")

	// new database
	newCmd.Flags().StringVarP(&app.DBName, "name", "n", config.MainDBName, "new database name")
	dbRootCmd.AddCommand(newCmd)
	// show database info
	infoCmd.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "output in JSON format")
	// remove database
	rmCmd.Flags().BoolVarP(&app.Flags.Menu, "menu", "m", false, "select database to remove (fzf compatible)")

	// backup
	backupUnlockCmd.Flags().BoolVarP(&app.Flags.Menu, "menu", "m", false,
		"select a backup to lock|unlock (fzf compatible)")
	backupCmd.AddCommand(BackupNewCmd, backupRmCmd, backupLockCmd, backupUnlockCmd)
	dbRootCmd.AddCommand(backupCmd)

	// remove
	removeCmd.AddCommand(dbRemoveCmd, bkRemoveCmd)
	dbRootCmd.AddCommand(removeCmd)

	dbRootCmd.AddCommand(dropCmd, rmCmd)

	return dbRootCmd
}
