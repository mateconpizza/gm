// Package health provides bookmark health verification and maintenance.
package health

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/health/wayback"
	"github.com/mateconpizza/gm/cmd/records"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd() *cobra.Command {
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

	cfg := config.New()
	f := healthCmd.Flags()
	f.BoolVarP(&cfg.Flags.Status, "status", "s", false, "check HTTP status of bookmark URLs")
	f.BoolVarP(&cfg.Flags.Update, "update", "u", false, "update bookmark metadata")
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "interactive menu mode using fzf")
	f.BoolVar(&cfg.Flags.Multiline, "multiline", false, "output in multiline format (fzf)")

	records.InitFilterFlags(healthCmd, cfg)

	healthCmd.AddCommand(wayback.NewCmd())

	return healthCmd
}

func checkerFunc(cmd *cobra.Command, args []string) error {
	cfg := config.New()
	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	bs, err := handler.Data(cmd.Context(), menuForRecords(cfg), r, args, cfg.Flags)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	})

	f := cfg.Flags
	switch {
	case f.Status: // FIX: remove
		return handler.CheckStatus(cmd.Context(), c, r, bs)
	case f.Update:
		return handler.Update(cmd.Context(), c, r, cfg, bs)
	}

	return handler.CheckStatus(cmd.Context(), c, r, bs)
}

func menuForRecords[T bookmark.Bookmark](cfg *config.Config) *menu.Menu[T] {
	return menu.New[T](
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithPreview(cfg.Cmd+" --name "+cfg.DBName+" records {1}"),
	)
}
