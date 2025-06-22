package cmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

func init() {
	removeCmd.AddCommand(dbRemoveCmd, bkRemoveCmd)
	Root.AddCommand(removeCmd)
}

var (
	// bkRemoveOtroCmd removes backups.
	bkRemoveCmd = &cobra.Command{
		Use:     "bk",
		Short:   "Remove one or more backups from local storage",
		Aliases: []string{"backup", "b", "backups"},
		RunE: func(_ *cobra.Command, _ []string) error {
			input := "s\n" // input for prompt, this will show menu to select brackups.
			c := ui.NewConsole(
				ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightGray))),
				ui.WithTerminal(terminal.New(
					terminal.WithReader(strings.NewReader(input)),
					terminal.WithWriter(io.Discard), // send output to null, show no prompt
				)),
			)

			c.F.Headerln(color.BrightRed("Removing").String() + " backups").Rowln().Flush()

			return handler.RemoveBackups(c, config.App.DBPath)
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
			c := ui.NewConsole(
				ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
				ui.WithTerminal(
					terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) })),
				),
			)

			return handler.RemoveRepo(c, config.App.DBPath)
		},
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
