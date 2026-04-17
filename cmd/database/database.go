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
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/files"
)

// NewCmd database management.
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "db",
		Aliases: []string{"database", "d"},
		Short:   "database ops",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			switch {
			case d.App.Flags.Vacuum:
				slog.Debug("database:", "vacuum", d.App.DBName)
				defer d.DB.Close()
				return d.DB.Vacuum(cmd.Context())

			case d.App.Flags.Reorder:
				slog.Debug("database: reordering bookmark IDs")
				defer d.DB.Close()

				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()
				return d.DB.ReorderIDs(ctx)
			}

			return cmd.Usage()
		},
	}

	f := c.Flags()
	f.SortFlags = false
	f.BoolVarP(&app.Flags.Vacuum, "vacuum", "X", false, "rebuilds the database file")
	f.BoolVarP(&app.Flags.Reorder, "reorder", "R", false, "reorder IDs")

	c.AddCommand(
		createCmd, newDatabaseRemoveCmd(app), newListCmd(app),
		newInfoCmd(app), newBackupCmd(app), newDropCmd(app),
		newLockCmd(app), newUnlockCmd(app))

	return c
}

func newInfoCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "info",
		Short:   "show info about a database",
		Aliases: []string{"i", "show"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.RepoInfo(d)
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "output in JSON format")

	return c
}

func newListCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls"},
		Short:   "list all databases",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.DatabasesTable(cmd.Context(), d.Console(), d.App.Path.Data)
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

func newDropCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:      "drop",
		Short:    "drop a database",
		PostRunE: dbDropPostFunc(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return handler.DropDatabase(d)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newLockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "lock",
		Short: "lock a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return handler.LockRepo(d, d.App.DBPath)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newUnlockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "unlock",
		Short:       "unlock a database",
		Annotations: cli.SkipDBCheckAnnotation,
		RunE: func(cmd *cobra.Command, args []string) error {
			d := deps.New(cmd.Context(),
				deps.WithApplication(app),
				deps.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
			)

			return handler.UnlockRepo(d, d.App.DBPath)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func dbDropPostFunc(app *application.App) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if !app.Git.Enabled {
			return nil
		}

		gr, err := git.NewRepo(app.DBPath)
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
