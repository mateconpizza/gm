package database

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
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
)

func newBackupRemoveCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Short:   "remove one or more backups",
		Aliases: []string{"backup", "b", "backups"},
		RunE: func(cmd *cobra.Command, args []string) error {
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

	cmdutil.FlagDBRequired(c, cfg)

	return c
}

func newDatabaseRemoveCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:      "rm",
		Aliases:  []string{"remove"},
		Short:    "remove a database",
		PostRunE: databaseRemovePostFunc,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return handler.RemoveRepo(a)
		},
	}

	cmdutil.FlagDBRequired(c, cfg)

	return c
}

func databaseRemovePostFunc(cmd *cobra.Command, _ []string) error {
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
