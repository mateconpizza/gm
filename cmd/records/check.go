// Package records provides bookmark health verification and maintenance.
package records

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/records/wayback"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func newCheckCmd(cfg *config.Config) *cobra.Command {
	checkCmd := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c"},
		Short:   "bookmark health",
		Example: `  # Check specific aspects
  gm records check --status
  gm records check --update

  # Check specific bookmarks
  gm records check golang.org --status
  gm records check --tag tutorial --status`,
		RunE: checkerFunc,
	}

	f := checkCmd.Flags()
	f.BoolVarP(&cfg.Flags.Status, "status", "s", false,
		"check HTTP status of bookmark URLs")
	f.BoolVarP(&cfg.Flags.Update, "update", "u", false,
		"update bookmark metadata (title|desc|tags)")
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false,
		"interactive menu mode using fzf")

	InitFilterFlags(checkCmd, cfg)

	checkCmd.AddCommand(wayback.NewCmd(cfg))

	return checkCmd
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

	m := handler.MenuSimple[bookmark.Bookmark](cfg,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s for checking status"))
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

	return cmd.Help()
}
