package cmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

// bkRemoveOtroCmd removes backups.
var bkRemoveCmd = &cobra.Command{
	Use:     "bk",
	Short:   "Remove a backup",
	Aliases: []string{"backup", "b", "backups"},
	RunE: func(_ *cobra.Command, _ []string) error {
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		t := terminal.New(
			// send 's' as 'select' to prompt user menu interface to select backups
			// from repo.
			terminal.WithReader(strings.NewReader("s\n")),
			// send the output to null, show no prompt
			terminal.WithWriter(io.Discard),
		)

		return handler.RemoveBackups(t, f, config.App.DBPath)
	},
}

// dbRemoveCmd remove a database.
var dbRemoveCmd = &cobra.Command{
	Use:     "db",
	Aliases: []string{"database", "d"},
	Short:   "Remove a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
		return handler.RemoveRepo(t, config.App.DBPath)
	},
}

// removeCmd databases/backups management.
var removeCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove databases/backups",
	Aliases: []string{"rm", "del"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Usage()
	},
}

func init() {
	removeCmd.AddCommand(dbRemoveCmd, bkRemoveCmd)
	rootCmd.AddCommand(removeCmd)
}
