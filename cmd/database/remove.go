package database

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	// bkRemoveOtroCmd removes backups.
	bkRemoveCmd = &cobra.Command{
		Use:     "bk",
		Short:   "Remove one or more backups from local storage",
		Aliases: []string{"backup", "b", "backups"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			input := "s\n" // input for prompt, this will show menu to select brackups.
			c := ui.NewConsole(
				ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightGray))),
				ui.WithTerminal(terminal.New(
					terminal.WithContext(cmd.Context()),
					terminal.WithReader(strings.NewReader(input)),
					terminal.WithWriter(io.Discard), // send output to null, show no prompt
				)),
			)

			c.F.Headerln(color.BrightRed("Removing").String() + " backups").Rowln().Flush()

			return handler.RemoveBackups(c, config.New())
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
			app := config.New()
			if len(args) > 0 {
				app.DBName = files.EnsureSuffix(args[0], ".db")
				app.DBPath = filepath.Join(app.Path.Data, app.DBName)
			}

			c := ui.NewConsole(
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
				ui.WithTerminal(
					terminal.New(
						terminal.WithContext(cmd.Context()),
						terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }),
					),
				),
			)

			if app.Flags.Menu {
				s, err := handler.SelectDatabase(app.Path.Data)
				if err != nil {
					return err
				}

				app.DBPath = s
			}

			return handler.RemoveRepo(c, app)
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
	app := config.New()
	if !app.Git.Enabled {
		return nil
	}

	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}
	if !gr.IsTracked() {
		return nil
	}

	return gr.Untrack("removed database")
}
