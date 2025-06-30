package database

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

func init() {
	cfg := config.App
	f := dbRootCmd.Flags()
	f.StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "database name")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	// new database
	DatabaseNewCmd.Flags().StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "new database name")

	// show database info
	databaseInfoCmd.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	// remove database
	databaseRmCmd.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "select database to remove (fzf)")

	dbRootCmd.AddCommand(
		databaseDropCmd, databaseInfoCmd, DatabaseNewCmd, databaseListCmd,
		databaseRmCmd, databaseLockCmd, databaseUnlockCmd,
	)
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
			cfg := config.App
			if cfg.Flags.JSON {
				c := ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))))
				return printer.RepoInfo(c, cfg.DBPath, cfg.Flags.JSON)
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

	// databaseListCmd lists the available databases.
	databaseListCmd = &cobra.Command{
		Use:     "list",
		Short:   "List databases",
		Aliases: []string{"ls", "l"},
		RunE: func(_ *cobra.Command, _ []string) error {
			return printer.DatabasesList(
				ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray)))),
				config.App.Path.Data,
			)
		},
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
