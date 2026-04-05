package database

import (
	"fmt"
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
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	// bkRemoveOtroCmd removes backups.
	bkRemoveCmd = &cobra.Command{
		Use:     "bk",
		Short:   "remove one or more backups from local storage",
		Aliases: []string{"backup", "b", "backups"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

			input := "s\n" // input for prompt, this will show the menu to select backups files.
			a := app.New(cmd.Context(),
				app.WithConfig(cfg),
				app.WithConsole(ui.NewConsole(
					ui.WithFrame(frame.New(frame.WithColorBorder(ansi.BrightBlack))),
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

			return handler.RemoveBackups(a)
		},
	}

	// dbRemoveCmd remove a database.
	dbRemoveCmd = &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove"},
		Short:   "remove a database from local storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}

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
)

func dbRemovePostFunc(cmd *cobra.Command, _ []string) error {
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
	if !gr.IsTracked() {
		return nil
	}

	return gr.Untrack("removed database")
}
