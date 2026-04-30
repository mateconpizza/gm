package cmd

import (
	"context"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/add"
	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/cmd/config"
	"github.com/mateconpizza/gm/cmd/database"
	"github.com/mateconpizza/gm/cmd/edit"
	gitCmd "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/cmd/notes"
	"github.com/mateconpizza/gm/cmd/open"
	"github.com/mateconpizza/gm/cmd/qrcmd"
	"github.com/mateconpizza/gm/cmd/rm"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/cmd/tag"
	urlcmd "github.com/mateconpizza/gm/cmd/url"
	"github.com/mateconpizza/gm/cmd/yank"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

// Setup registers all application commands with the CLI.
func Setup(root *cobra.Command, app *application.App) {
	cmds := []func(*application.App) *cobra.Command{
		add.NewCmd,
		edit.NewCmd,
		rm.NewCmd,
		open.NewCmd,
		yank.NewCmd,
		notes.NewCmd,
		qrcmd.NewCmd,
		urlcmd.NewCmd,
		tag.NewCmd,
		database.NewCmd,
		gitCmd.NewCmd,
		config.NewCmd,
		setup.NewCmd,
	}
	for i := range cmds {
		c := cmds[i](app)
		cmdutil.DisableFlagSorting(c)
		root.AddCommand(c)
	}
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

func registerCleanups(_ *application.App) {
	// close all open connections
	cleanup.Register(func() error {
		db.Shutdown()
		return nil
	})

	// synchronize the repository state on shutdown.
	// cleanup.Register(func() error {
	// FIX: this make exit code 130, disabled for now.
	// 	slog.Debug("synchronize the repository state on shutdown")
	// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	// 	defer cancel()
	// 	return git.Sync(ctx, app, "cleanup: sync pending changes")
	// })
}

func registerRootFlags(c *cobra.Command, app *application.App) {
	c.Flags().SortFlags = false

	// local
	// limit results (head/tail semantics)
	cmdutil.FlagsFilter(c, app)
	// interactive mode
	cmdutil.FlagMenu(c, app)
	// output formatting
	cmdutil.FlagOutput(c, app, formatter.ValidFormats())
	// sorting strategy (domain-specific ordering options)
	cmdutil.FlagSort(c, app, handler.SortSupported)
	// field selection for output projection
	fields := []string{"id", "url", "title", "tags", "desc"}
	cmdutil.FlagFields(c, app, strings.Join(fields, ", "))

	// global
	g := c.PersistentFlags()
	// database selection
	g.StringVar(&app.DBName, "db", application.MainDBName, "database name")
	// output colorization policy
	g.StringVar(&app.Flags.ColorStr, "color", "always", "colorize output: always, never")
	// non-interactive confirmation
	g.BoolVarP(&app.Flags.Yes, "yes", "y", false, "assume yes")
	// force execution even if safeguards would prevent it
	g.BoolVar(&app.Flags.Force, "force", false, "force action")
	// verbosity level
	g.CountVarP(&app.Flags.Verbose, "verbose", "v", "increase verbosity (-v, -vv, -vvv)")

	// hidden
	g.Bool("help", false, "")
	_ = g.MarkHidden("help")
	g.StringVar(&app.Flags.Preview, "preview", "", "")
	_ = g.MarkHidden("preview")
}
