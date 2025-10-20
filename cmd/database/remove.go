package database

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	// bkRemoveOtroCmd removes backups.
	bkRemoveCmd = &cobra.Command{
		Use:     "bk",
		Short:   "Remove one or more backups from local storage",
		Aliases: []string{"backup", "b", "backups"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			input := "s\n" // input for prompt, this will show the menu to select backups files.
			a := app.New(cmd.Context(),
				app.WithConfig(config.New()),
				app.WithConsole(ui.NewConsole(
					ui.WithFrame(frame.New(frame.WithColorBorder(frame.ColorGray))),
					ui.WithTerminal(terminal.New(
						terminal.WithContext(cmd.Context()),
						terminal.WithInterruptFn(func(err error) {
							db.Shutdown()
							sys.ErrAndExit(err)
						}),
						terminal.WithReader(strings.NewReader(input)),
						terminal.WithWriter(io.Discard), // send output to null, show no prompt
					)),
				)),
			)

			c := a.Console()
			c.Frame().Headerln(c.Palette().BrightRed("Removing") + " backups").Rowln().Flush()

			return handler.RemoveBackups(a)
		},
	}

	// dbRemoveCmd remove a database.
	dbRemoveCmd = &cobra.Command{
		Use:     "db",
		Aliases: []string{"database", "d"},
		Short:   "Remove one or more databases from local storage",
		Example: `  gm rm db -n dbName
  gm rm db -n dbName --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.New()
			if len(args) > 0 {
				cfg.DBName = files.EnsureSuffix(args[0], ".db")
				cfg.DBPath = filepath.Join(cfg.Path.Data, cfg.DBName)
			}

			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
					db.Shutdown()
					sys.ErrAndExit(err)
				})),
			)

			if cfg.Flags.Menu {
				s, err := handler.SelectDatabase(a, cfg.Path.Data)
				if err != nil {
					return err
				}

				cfg.DBPath = s
			}

			return handler.RemoveRepo(a)
		},
		PostRunE: dbRemovePostFunc,
	}

	// removeCmd databases/backups management.
	removeCmd = &cobra.Command{
		Use:     "remove",
		Short:   "Remove databases/backups",
		Aliases: []string{"rm", "del"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Usage()
		},
	}
)

func dbRemovePostFunc(_ *cobra.Command, _ []string) error {
	cfg := config.New()
	if !cfg.Git.Enabled {
		return nil
	}

	gr, err := git.NewRepo(cfg.DBPath)
	if err != nil {
		return err
	}
	if !gr.IsTracked() {
		return nil
	}

	return gr.Untrack("removed database")
}
