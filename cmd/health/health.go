// Package health provides bookmark health verification and maintenance.
package health

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/health/wayback"
	"github.com/mateconpizza/gm/cmd/records"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	healthCmd := &cobra.Command{
		Use:     "health [query]",
		Aliases: []string{"h", "check", "verify"},
		Short:   "Bookmark health",
		Example: `  # Check specific aspects
  gm health --status
  gm health --update

  # Check specific bookmarks
  gm health golang.org --status
  gm health -t tutorial --status`,
		RunE: checkerFunc,
	}

	f := healthCmd.Flags()
	f.BoolVarP(&cfg.Flags.Status, "status", "s", false, "check HTTP status of bookmark URLs")
	f.BoolVarP(&cfg.Flags.Update, "update", "u", false, "update bookmark metadata")
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "interactive menu mode using fzf")
	f.BoolVar(&cfg.Flags.Multiline, "multiline", false, "output in multiline format (fzf)")

	records.InitFilterFlags(healthCmd, cfg)

	healthCmd.AddCommand(wayback.NewCmd(cfg))

	return healthCmd
}

func checkerFunc(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	a := app.New(cmd.Context(),
		app.WithDB(r),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	m := handler.MenuMainForRecords[bookmark.Bookmark](cfg)
	bs, err := handler.Data(a, m, args)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	f := a.Cfg.Flags
	switch {
	case f.Status: // FIX: remove
		return handler.CheckStatus(a, bs)
	case f.Update:
		return handler.Update(a, bs)
	}

	return handler.CheckStatus(a, bs)
}
