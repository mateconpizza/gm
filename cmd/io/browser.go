package io

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Import from browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := db.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		c := ui.NewConsole(
			ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
				r.Close()
				sys.ErrAndExit(err)
			}))),
		)

		return port.Browser(c, r)
	},
}
