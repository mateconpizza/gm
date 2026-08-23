package database

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

func newBackupRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Short:   "remove one or more backups",
		Aliases: []string{"backup", "b", "backups"},
		Example: app.Example(`  $ {cmd} db backup rm
  $ {cmd} db backup rm --db work
  $ {cmd} db backup rm --yes`),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := "select\n" // input for prompt, this will show the menu for selecting backup files.
			d := deps.New(
				deps.WithApplication(app),
				deps.WithConsole(ui.NewConsole(
					ui.WithFrame(frame.New(frame.WithColorBorder(ansi.Gray))),
					ui.WithTerminal(terminal.New(
						terminal.WithInterruptFn(func(err error) {
							db.Shutdown()
							sys.ErrAndExit(err)
						}),
						terminal.WithReader(strings.NewReader(input)),
						terminal.WithWriter(io.Discard), // send output to null, show no prompt
					)),
				)),
			)

			return dbops.RemoveBackups(cmd.Context(), d)
		},
	}

	return c
}

func newDatabaseRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove"},
		Short:   "remove a database",
		Example: app.Example(`  $ {cmd} db rm --db {db}`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd, args, handler.RemoveAndUntrack)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}
