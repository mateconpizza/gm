// Package database handles bookmarks database management operations.
package database

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// NewCmd database management.
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "db",
		Aliases: []string{"database", "d", "repo"},
		Short:   "database operations",
	}

	c.AddCommand(
		newAddCmd(app),            // create
		newUseCmd(app),            // switch context
		newCurrentCmd(app),        // inspect current
		newListCmd(app),           // inspect all
		newStatsCmd(app),          // inspect one
		newBackupCmd(app),         // safe management
		newDatabaseRemoveCmd(app), // destructive
		newDropCmd(app),           // most destructive
		newLockCmd(app),           // restrict access
		newUnlockCmd(app),         // restore access
		newImportCmd(app),         // data in
		newExportCmd(app),         // data out
		newReorderCmd(app),        // reorder IDs
		newVacuumCmd(app),         // compact database file
	)

	return c
}

func newStatsCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "stats",
		Short:       "show database stats",
		Aliases:     []string{"i", "show", "info"},
		Annotations: cli.SkipGitSync,
		Example: app.Example(`  $ {cmd} db stats
  $ {cmd} db stats --db work
  $ {cmd} db stats --json
  $ {cmd} db stats --db {db} --json`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// FIX: add struct for building the RepoStats.
			// - enable to port to JSON
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.RepoStats(cmd.Context(), d)
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "output in JSON format")

	return c
}

func newListCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "list",
		Aliases:     []string{"l", "ls"},
		Short:       "list all databases",
		Annotations: cli.SkipGitSync,
		Example:     app.Example(`  $ {cmd} db list`),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.DatabasesTable(cmd.Context(), d.Console(), app.Path.Home(), app.DBName)
		},
	}

	cmdutil.HideFlag(c, "force", "yes")

	return c
}

func newAddCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "add",
		Short:   "add a database",
		Aliases: []string{"create", "new"},
		Example: app.Example(`  $ {cmd} db add --db <name>
  $ {cmd} db new --db <name>
  $ {cmd} db create --db <name>`),
		Annotations: cli.SkipDBCheck,
		RunE: func(cmd *cobra.Command, args []string) error {
			if files.Exists(app.Path.DB()) {
				return fmt.Errorf("%w: %q", db.ErrDBExists, app.DBName)
			}

			return setup.InitCmd.RunE(cmd, args)
		},
		PostRunE: setup.InitCmd.PostRunE,
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newDropCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "drop",
		Short: "drop a database",
		Example: app.Example(`  $ {cmd} db drop --db {db}
			$ {cmd} db drop --db {db} --yes
			$ {cmd} db drop --db work --yes`),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			ctx := cmd.Context()
			if err := dbops.Drop(ctx, d); err != nil {
				return err
			}

			return gitops.Drop(ctx, app, d.Console())
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newLockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "lock",
		Short:       "lock a database",
		Annotations: cli.SkipGitSync,
		Example: app.Example(`  $ {cmd} db lock --db {db}
  $ {cmd} db lock --db work`),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return dbops.Lock(cmd.Context(), d, app.Path.DB())
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newUnlockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "unlock",
		Short: "unlock a database",
		Example: app.Example(`  $ {cmd} db unlock --db {db}
  $ {cmd} db unlock --db work`),
		Annotations: cli.ChainAnnotations(cli.SkipDBCheck, cli.SkipGitSync),
		RunE: func(cmd *cobra.Command, args []string) error {
			d := deps.New(
				deps.WithApplication(app),
				deps.WithConsole(ui.DefaultConsole),
			)

			return dbops.Unlock(cmd.Context(), d, app.Path.DB())
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newUseCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "use [name]",
		Short:       "set default database",
		Annotations: cli.ChainAnnotations(cli.SkipDBCheck, cli.SkipGitSync),
		Example: app.Example(`  $ {cmd} db use <name>
  $ {cmd} db use default`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return dbops.SetDefault(cmd.Context(), app, args[0])
		},
	}

	return c
}

func newCurrentCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:         "current",
		Short:       "current default",
		Annotations: cli.ChainAnnotations(cli.SkipDBCheck, cli.SkipGitSync),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(app.DBName)
			return nil
		},
	}
}

func newReorderCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "reorder",
		Short: "renumber bookmark IDs sequentially",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbops.ReorderDatabase(cmd.Context(), app)
		},
	}
}

func newVacuumCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "vacuum",
		Short: "compact and rebuild the database file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return dbops.VacuumDatabase(cmd.Context(), app)
		},
	}
}
