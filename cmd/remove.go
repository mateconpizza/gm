package cmd

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

// bkRemoveOtroCmd removes backups.
var bkRemoveCmd = &cobra.Command{
	Use:     "bk",
	Short:   "Remove one or more backups from local storage",
	Aliases: []string{"backup", "b", "backups"},
	RunE: func(_ *cobra.Command, _ []string) error {
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		input := "s\n" // input for prompt, this will show menu to select brackups.
		t := terminal.New(
			terminal.WithReader(strings.NewReader(input)),
			terminal.WithWriter(io.Discard), // send output to null, show no prompt
		)

		return handler.RemoveBackups(t, f, config.App.DBPath)
	},
}

// dbRemoveCmd remove a database.
var dbRemoveCmd = &cobra.Command{
	Use:     "db",
	Aliases: []string{"database", "d"},
	Short:   "Remove one or more databases from local storage",
	Example: `  gm rm db -n dbName
  gm rm db -n dbName --force`,
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
