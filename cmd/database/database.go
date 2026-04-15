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
	"github.com/mateconpizza/gm/pkg/files"
)

// NewCmd database management.
func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "db",
		Aliases: []string{"database", "d"},
		Short:   "database ops",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			switch {
			case a.Cfg.Flags.Vacuum:
				slog.Debug("database:", "vacuum", a.Cfg.DBName)
				defer a.DB.Close()
				return a.DB.Vacuum(cmd.Context())

			case a.Cfg.Flags.Reorder:
				slog.Debug("database: reordering bookmark IDs")
				defer a.DB.Close()

				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()
				return a.DB.ReorderIDs(ctx)
			}

			return cmd.Usage()
		},
	}

	f := c.Flags()
	f.SortFlags = false
	f.BoolVarP(&cfg.Flags.Vacuum, "vacuum", "X", false, "rebuilds the database file")
	f.BoolVarP(&cfg.Flags.Reorder, "reorder", "R", false, "reorder IDs")

	c.AddCommand(
		createCmd, newDatabaseRemoveCmd(cfg), newListCmd(cfg),
		newInfoCmd(cfg), newBackupCmd(cfg), newDropCmd(cfg),
		newLockCmd(cfg), newUnlockCmd(cfg))

	return c
}

func newInfoCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "info",
		Short:   "show info about a database",
		Aliases: []string{"i", "show"},
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.RepoInfo(a)
		},
	}

	c.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false, "output in JSON format")

	return c
}

func newListCmd(_ *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls"},
		Short:   "list all databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.DatabasesTable(cmd.Context(), a.Console(), a.Cfg.Path.Data)
		},
	}
	return c
}

var createCmd = &cobra.Command{
	Use:               "create",
	Short:             "create a database",
	Aliases:           []string{"add"},
	Example:           `  gm db create -n myDb`,
	Annotations:       cli.SkipDBCheckAnnotation,
	PersistentPreRunE: setup.InitCmd.PersistentPreRunE,
	RunE:              setup.InitCmd.RunE,
	PostRunE:          setup.InitCmd.PostRunE,
}

func newDropCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:      "drop",
		Short:    "drop a database",
		PostRunE: dbDropPostFunc(cfg),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return handler.DropDatabase(a)
		},
	}

	cmdutil.FlagDBRequired(c, cfg)

	return c
}

func newLockCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "lock",
		Short: "lock a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return handler.LockRepo(a, a.Cfg.DBPath)
		},
	}

	cmdutil.FlagDBRequired(c, cfg)

	return c
}

func newUnlockCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
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

			return handler.UnlockRepo(a, a.Cfg.DBPath)
		},
	}

	cmdutil.FlagDBRequired(c, cfg)

	return c
}

func dbDropPostFunc(cfg *config.Config) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
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
}
