package cmd

import (
	"context"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/add"
	"github.com/mateconpizza/gm/cmd/archive"
	"github.com/mateconpizza/gm/cmd/check"
	"github.com/mateconpizza/gm/cmd/clean"
	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/config"
	"github.com/mateconpizza/gm/cmd/database"
	"github.com/mateconpizza/gm/cmd/edit"
	gitCmd "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/cmd/io/in"
	"github.com/mateconpizza/gm/cmd/io/out"
	"github.com/mateconpizza/gm/cmd/notes"
	"github.com/mateconpizza/gm/cmd/open"
	"github.com/mateconpizza/gm/cmd/qrcmd"
	"github.com/mateconpizza/gm/cmd/rm"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/cmd/tag"
	"github.com/mateconpizza/gm/cmd/yank"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

// TODO: let the user set the default database.
// - [ ] gm db use <name> (this will set it as default)?

// FIX: keymap 'toggle-preview' does not respect the user configuration.
// If the user sets the keybind to 'hidden', it will be overwritten when:
// - 'register' -> 'menu/keymap.go'
// or
// - 'buildPreviewArgs' -> 'menu/builder.go'

// Setup registers all application commands with the CLI.
func Setup(root *cobra.Command, app *application.App) {
	root.AddCommand(
		add.NewCmd(app),
		edit.NewCmd(app),
		rm.NewCmd(app),
		open.NewCmd(app),
		yank.NewCmd(app),
		notes.NewCmd(app),
		qrcmd.NewCmd(app),
		check.NewCmd(app),
		tag.NewCmd(app),
		clean.NewCmd(app),
		archive.NewCmd(app),
		database.NewCmd(app),
		gitCmd.NewCmd(app),
		config.NewCmd(app),
		in.NewCmd(app),
		out.NewCmd(app),
		setup.NewCmd(),
	)
}

func initAppConfig(ctx context.Context, app *application.App) {
	app.Flags.Color = app.Flags.ColorStr == "always" &&
		!terminal.IsPiped() &&
		!terminal.NoColorEnv()

	application.SetVerbosity(app.Flags.Verbose)

	// load config from YAML
	if err := app.Load(); err != nil {
		slog.Error("loading config", "err", err)
	}

	// enable global color
	if !app.Flags.Color {
		ansi.DisableColor()
		frame.DisableColor()
	}

	// terminal interactive mode
	terminal.NonInteractiveMode(app.Flags.Yes)

	// git config
	git.SetConfig(ctx, app)
}

func registerCleanups(app *application.App) {
	// close all open connections
	cleanup.Register(func() error {
		db.Shutdown()
		return nil
	})

	// synchronize the repository state on shutdown.
	cleanup.Register(func() error {
		slog.Debug("synchronize the repository state on shutdown")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		return git.Sync(ctx, app, "cleanup: sync pending changes")
	})
}

func registerFlags(c *cobra.Command, app *application.App) {
	// local
	cmdutil.FlagOutput(c, app)
	cmdutil.FlagFields(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagMenu(c, app)

	// global
	g := c.PersistentFlags()
	g.StringVar(&app.DBName, "db", application.MainDBName, "database name")
	g.StringVar(&app.Flags.ColorStr, "color", "always", "output with colors [always|never]")
	g.BoolVar(&app.Flags.Force, "force", false, "force action")
	g.BoolVarP(&app.Flags.Yes, "yes", "y", false, "assume yes")
	g.CountVarP(&app.Flags.Verbose, "verbose", "v", "increase verbosity (-v, -vv, -vvv)")
	g.Bool("help", false, "")
	_ = g.MarkHidden("help")
	g.StringVar(&app.Flags.Preview, "preview", "", "")
	_ = g.MarkHidden("preview")

	c.Flags().BoolVarP(&app.Flags.Version, "version", "V", false, "version for "+app.Cmd)
}
