package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
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

type dbFlagsType struct {
	reorder bool // reorder tables IDs
	list    bool // list items
	lock    bool // lock a database
	unlock  bool // unlock a database
	vacuum  bool // rebuilds the database file
	info    bool // item info
}

var dbFlags = &dbFlagsType{}

func init() {
	cfg := config.App
	f := dbRootCmd.Flags()
	f.StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "database name")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	f.BoolVarP(&dbFlags.vacuum, "vacuum", "X", false, "rebuilds the database file")
	f.BoolVarP(&dbFlags.reorder, "reorder", "R", false, "reorder IDs")
	f.BoolVarP(&dbFlags.lock, "lock", "L", false, "lock a database")
	f.BoolVarP(&dbFlags.unlock, "unlock", "U", false, "unlock a database")
	f.BoolVarP(&dbFlags.list, "list", "l", false, "list databases")
	f.BoolVarP(&dbFlags.info, "info", "i", false, "database information")

	// new database
	DatabaseNewCmd.Flags().StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "new database name")

	// show database info
	databaseInfoCmd.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	// remove database
	databaseRmCmd.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "select database to remove (fzf)")

	dbRootCmd.AddCommand(databaseDropCmd, DatabaseNewCmd, databaseRmCmd)
	cmd.Root.AddCommand(dbRootCmd)
}

var (
	// dbRootCmd database management.
	dbRootCmd = &cobra.Command{
		Use:               "db",
		Aliases:           []string{"database", "d"},
		Short:             "Database management",
		PersistentPreRunE: cmd.RequireDatabase,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case config.App.Flags.JSON:
				c := ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))))
				return printer.RepoInfo(c, config.App.DBPath, config.App.Flags.JSON)

			case dbFlags.vacuum:
				slog.Debug("database:", "vacuum", config.App.DBName)
				r, err := db.New(config.App.DBPath)
				if err != nil {
					return fmt.Errorf("backup: %w", err)
				}
				defer r.Close()

				return r.Vacuum(context.Background())

			case dbFlags.reorder:
				slog.Debug("database: reordering bookmark IDs")
				r, err := db.New(config.App.DBPath)
				if err != nil {
					return fmt.Errorf("backup: %w", err)
				}
				defer r.Close()

				return dbtask.DeleteAndReorder(context.Background(), r)

			case dbFlags.lock:
				return databaseLockCmd.RunE(cmd, args)
			case dbFlags.unlock:
				return databaseUnlockCmd.RunE(cmd, args)
			case dbFlags.list:
				return printer.DatabasesTable(config.App.Path.Data)
			case dbFlags.info:
				return databaseInfoCmd.RunE(cmd, args)
			}

			return cmd.Usage()
		},
	}

	// DatabaseNewCmd initialize a new bookmarks database.
	DatabaseNewCmd = &cobra.Command{
		Use:               "new",
		Short:             cmd.InitCmd.Short,
		Example:           `  gm db new -n newDBName`,
		Annotations:       cmd.SkipDBCheckAnnotation,
		PersistentPreRunE: cmd.InitCmd.PersistentPreRunE,
		RunE:              cmd.InitCmd.RunE,
		PostRunE:          cmd.InitCmd.PostRunE,
	}

	// databaseDropCmd drops a database.
	databaseDropCmd = &cobra.Command{
		Use:      "drop",
		Short:    "Drop a database",
		RunE:     dbDropFunc,
		PostRunE: dbDropPostFunc,
	}

	// databaseInfoCmd shows information about a database.
	databaseInfoCmd = &cobra.Command{
		Use:     "info",
		Short:   "Show information about a database",
		Aliases: []string{"i", "show"},
		RunE: func(_ *cobra.Command, _ []string) error {
			return printer.RepoInfo(
				ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray)))),
				config.App.DBPath,
				config.App.Flags.JSON,
			)
		},
	}

	// databaseRmCmd remove a database.
	databaseRmCmd = &cobra.Command{
		Use:      "rm",
		Short:    dbRemoveCmd.Short,
		Aliases:  []string{"r", "remove"},
		Example:  dbRemoveCmd.Example,
		RunE:     dbRemoveCmd.RunE,
		PostRunE: dbRemoveCmd.PostRunE,
	}

	databaseLockCmd = &cobra.Command{
		Use:   "lock",
		Short: "Lock a database",
		RunE: func(_ *cobra.Command, _ []string) error {
			c := ui.NewConsole(
				ui.WithTerminal(
					terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) })),
				),
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
			)

			return handler.LockRepo(c, config.App.DBPath)
		},
	}

	databaseUnlockCmd = &cobra.Command{
		Use:         "unlock",
		Short:       "Unlock a database",
		Annotations: cmd.SkipDBCheckAnnotation,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := ui.NewConsole(
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Purple))),
				ui.WithTerminal(
					terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) })),
				),
			)

			return handler.UnlockRepo(c, config.App.DBPath)
		},
	}
)

func dbDropFunc(_ *cobra.Command, _ []string) error {
	r, err := db.New(config.App.DBPath)
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

	return handler.DroppingDB(c, r)
}

func dbDropPostFunc(_ *cobra.Command, _ []string) error {
	cfg := config.App
	if !git.IsInitialized(cfg.Git.Path) {
		return nil
	}

	gr, err := git.NewRepo(cfg.DBPath)
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
