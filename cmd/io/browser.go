package io

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "import from browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.FromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		r, err := db.New(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		a := app.New(cmd.Context(),
			app.WithDB(r),
			app.WithConfig(cfg),
			app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
				db.Shutdown()
				sys.ErrAndExit(err)
			})),
		)

		return port.Browser(a)
	},
}
