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

	app := config.New()
	f := healthCmd.Flags()
	f.BoolVarP(&app.Flags.Status, "status", "s", false, "check HTTP status of bookmark URLs")
	f.BoolVarP(&app.Flags.Update, "update", "u", false, "update bookmark metadata")

	records.InitFilterFlags(healthCmd, app)

	healthCmd.AddCommand(wayback.NewCmd())

	return healthCmd
}

func checkerFunc(cmd *cobra.Command, args []string) error {
	app := config.New()
	r, err := db.New(app.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	bs, err := handler.Data(menuForRecords(app), r, args, app.Flags)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	c := ui.NewDefaultConsole(func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	})

	f := app.Flags
	switch {
	case f.Status: // FIX: remove
		return handler.CheckStatus(c, r, bs)
	case f.Update:
		return handler.Update(c, r, app, bs)
	}

	return handler.CheckStatus(c, r, bs)
}

func menuForRecords[T bookmark.Bookmark](app *config.Config) *menu.Menu[T] {
	return menu.New[T](
		menu.WithSettings(app.Menu.Settings),
		menu.WithPreview(app.Cmd+" --name "+app.DBName+" records {1}"),
	)
}
