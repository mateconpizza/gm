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
	"github.com/mateconpizza/gm/pkg/db"
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

			r, err := d.Repository()
			if err != nil {
				return err
			}

			switch {
			case app.Flags.Vacuum:
				slog.Debug("database:", "vacuum", app.DBName)
				defer r.Close()
				return r.Vacuum(cmd.Context())

			case app.Flags.Reorder:
				slog.Debug("database: reordering bookmark IDs")
				defer r.Close()

				ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
				defer cancel()
				return r.ReorderIDs(ctx)
			}

			return cmd.Usage()
		},
	}

	f := c.Flags()
	f.SortFlags = false
	f.BoolVarP(&app.Flags.Vacuum, "vacuum", "X", false,
		"compact and rebuild the database file")
	f.BoolVarP(&app.Flags.Reorder, "reorder", "R", false,
		"renumber bookmark IDs sequentially")

	cmdutil.HideInheritedFlags(c)

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
	)

	return c
}

func newStatsCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "stats",
		Short:   "show database stats",
		Aliases: []string{"i", "show", "info"},
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return printer.RepoStats(d)
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false, "output in JSON format")

	return c
}

func newListCmd(app *application.App) *cobra.Command {
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

			return printer.DatabasesTable(cmd.Context(), d.Console(), app.Path.Data, app.DBName)
		},
	}
	return c
}

func newAddCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "add",
		Short:       "add a database",
		Aliases:     []string{"create", "new"},
		Example:     `  gm db add --db myDb`,
		Annotations: cli.SkipDBCheck,
		RunE:        setup.InitCmd.RunE,
		PostRunE:    setup.InitCmd.PostRunE,
	}

	cmdutil.FlagDBRequired(c, app)

	return c
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

			return handler.LockRepo(d, app.Path.Database)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newUnlockCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "unlock",
		Short:       "unlock a database",
		Annotations: cli.SkipDBCheck,
		RunE: func(cmd *cobra.Command, args []string) error {
			d := deps.New(
				cmd.Context(),
				deps.WithApplication(app),
				deps.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })),
			)

			return handler.UnlockRepo(d, app.Path.Database)
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
		Example: `  gm db use <name>
  # restore to default
  gm db use default`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			filename := files.StripSuffixes(args[0])
			if filename == "" {
				return fmt.Errorf("%w: %q", handler.ErrInvalidOption, filename)
			}

			if filename == "default" {
				filename = application.MainDBName
			}
			if err := app.SetDatabase(filename); err != nil {
				return err
			}

			r, err := db.New(cmd.Context(), app.Path.Database)
			if err != nil {
				return err
			}
			defer r.Close()

			app.Flags.Force = true

			return app.WriteConfig()
		},
	}

	return c
}

func newCurrentCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "current default",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(app.DBName)
			return nil
		},
	}
}

func dbDropPostFunc(app *application.App) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if !app.Git.Enabled {
			return nil
		}

		gr, err := git.NewRepo(app.Path.Database)
		if err != nil {
			return err
		}
		if !gr.IsTracked() || !files.Exists(gr.Loc.DBPath) {
			return nil
		}

		// FIX: inject console
		c := ui.NewConsole(ui.WithDefaultTerminal(cmd.Context(), func(err error) { sys.ErrAndExit(err) }))

		if err := gr.Drop("dropped"); err != nil {
			return err
		}

		fmt.Fprintln(c.Writer(), c.SuccessMesg("database dropped"))

		if !c.Confirm("Untrack database?", "n") {
			return nil
		}

		if err := gr.Untrack(fmt.Sprintf("[%s] remove tracking", gr.Loc.Name)); err != nil {
			return err
		}

		fmt.Fprintln(c.Writer(), c.SuccessMesg("database untracked"))

		return nil
	}
}
