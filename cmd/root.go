// Package cmd contains the core commands and initialization logic for the
// application.
package cmd

import (
	"context"
	"log/slog"

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
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// TODO: let the user set the default database.
// - [ ] gm db use <name> (this will set it as default)?

// FIX: keymap 'toggle-preview' does not respect the user configuration.
// If the user sets the keybind to 'hidden', it will be overwritten when:
// - 'register' -> 'menu/keymap.go'
// or
// - 'buildPreviewArgs' -> 'menu/builder.go'

// NewRootCmd is the main command.
func NewRootCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:               app.Cmd + " [query]",
		Args:              cobra.MinimumNArgs(0),
		SilenceUsage:      true,
		PersistentPreRunE: cli.ChainHooks(cli.HookInjectApp(app), cli.HookEnsureDatabase),
		Version:           cli.PrettyVersion(app.Name, app.Info.Version),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuMainForRecords[bookmark.Bookmark](app)
			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				switch {
				case d.App.Flags.Format != "":
					return printer.Display(d.Console(), d.App.Flags.Format, bs)
				default:
					return printer.Records(d.Console(), bs)
				}
			})
		},
	}

	cobra.AddTemplateFunc("hasFlags", cmdutil.HasFlags)

	c.SetUsageTemplate(cmdutil.UsageTemplate)
	c.PersistentFlags().SortFlags = false

	// local
	cmdutil.FlagFormat(c, app)
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

	cobra.OnInitialize(func() {
		app.Initialize()
		initAppConfig(c.Context(), app)
	})

	// cmd settings
	c.CompletionOptions.HiddenDefaultCmd = true
	c.SilenceErrors = true
	c.DisableSuggestions = true
	c.SuggestionsMinimumDistance = 1
	c.SetHelpCommand(&cobra.Command{Hidden: true})
	cobra.EnableCommandSorting = false
	cobra.EnableTraverseRunHooks = true

	registerCleanups(app)

	return c
}

func initAppConfig(ctx context.Context, app *application.App) {
	app.Flags.Color = app.Flags.ColorStr == "always" &&
		!terminal.IsPiped() &&
		!terminal.NoColorEnv()

	application.SetVerbosity(app.Flags.Verbose)

	// load config from YAML
	if err := application.Load(app); err != nil {
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

// Execute executes the provided root command and exits on error.
func Execute(r *cobra.Command) error {
	ctx, stop := sys.WithSignalContext(context.Background())
	defer stop()

	return r.ExecuteContext(ctx)
}

func registerCleanups(_ *application.App) {
	// close all open connections
	cleanup.Register(func() error {
		db.Shutdown()
		return nil
	})
}
