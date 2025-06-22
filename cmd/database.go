package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

func init() {
	cfg := config.App
	f := dbCmd.Flags()
	f.StringVarP(&cfg.DBName, "name", "n", config.DefaultDBName, "database name")
	f.BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	// new database
	databaseNewCmd.Flags().StringVarP(&cfg.DBName, "name", "n", "", "new database name")
	_ = databaseNewCmd.MarkFlagRequired("name")

	// show database info
	databaseInfoCmd.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	// remove database
	databaseRmCmd.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "select database to remove (fzf)")

	dbCmd.AddCommand(
		databaseDropCmd, databaseInfoCmd, databaseNewCmd, databaseListCmd,
		databaseRmCmd, databaseLockCmd, databaseUnlockCmd,
	)
	Root.AddCommand(dbCmd)
}

var (
	// dbCmd database management.
	dbCmd = &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Database management",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "unlock" {
				return nil
			}

			return EnsureDatabaseExistence(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.App
			if cfg.Flags.JSON {
				c := ui.NewConsole(ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))))
				return printer.RepoInfo(c, cfg.DBPath, cfg.Flags.JSON)
			}

			return cmd.Usage()
		},
	}

	// databaseNewCmd initialize a new bookmarks database.
	databaseNewCmd = &cobra.Command{
		Use:   "new",
		Short: initCmd.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if initCmd.PersistentPreRunE != nil {
				if err := initCmd.PersistentPreRunE(cmd, args); err != nil {
					return fmt.Errorf("%w", err)
				}
			}

			return initCmd.RunE(cmd, args)
		},
	}

	// databaseDropCmd drops a database.
	databaseDropCmd = &cobra.Command{
		Use:   "drop",
		Short: "Drop a database",
		RunE:  dbDropFunc,
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
		Use:     "rm",
		Short:   dbRemoveCmd.Short,
		Aliases: []string{"r", "remove"},
		RunE:    dbRemoveCmd.RunE,
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
		Use:   "unlock",
		Short: "Unlock a database",
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

	if git.IsInitialized(config.App.Path.Git) &&
		git.IsTracked(config.App.Path.Git, r.Cfg.Fullpath()) {
		g, err := handler.NewGit(config.App.Path.Git)
		if err != nil {
			return nil
		}

		g.Tracker.SetCurrent(g.NewRepo(config.App.DBPath))

		if err := handler.GitDropRepo(g, "Dropped"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := handler.DroppingDB(c, r); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
