// Package cmd contains the core commands and initialization logic for the
// application.
package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/create"
	"github.com/mateconpizza/gm/cmd/database"
	gitCmd "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/cmd/io"
	"github.com/mateconpizza/gm/cmd/records"
	"github.com/mateconpizza/gm/cmd/settings"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/cmd/tags"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// NewRootCmd is the main command.
func NewRootCmd(app *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:               app.Cmd,
		Short:             app.Info.Title,
		Long:              app.Info.Desc,
		Args:              cobra.MinimumNArgs(0),
		SilenceUsage:      true,
		PersistentPreRunE: cli.HookEnsureDatabase,
		RunE:              records.Cmd,
		Version:           cli.PrettyVersion(app.Name, app.Info.Version),
	}

	// Global flags
	cmd.PersistentFlags().StringVarP(&app.DBName, "name", "n", config.MainDBName,
		"database name")
	cmd.PersistentFlags().StringVar(&app.Flags.ColorStr, "color", "always",
		"output with pretty colors [always|never]")
	cmd.PersistentFlags().CountVarP(&app.Flags.Verbose, "verbose", "v",
		"increase verbosity (-v, -vv, -vvv)")
	cmd.PersistentFlags().BoolVar(&app.Flags.Force, "force", false,
		"force action | don't ask confirmation")
	_ = cmd.PersistentFlags().MarkHidden("help")

	// Initialize flags for records commands
	records.InitFlags(cmd, app)

	cobra.OnInitialize(func() {
		app.Initialize()
		initConfig(app)
	})

	// cmd settings
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.SilenceErrors = true
	cmd.DisableSuggestions = true
	cmd.SuggestionsMinimumDistance = 1
	cobra.EnableCommandSorting = false
	cobra.EnableTraverseRunHooks = true

	return cmd
}

func initConfig(cfg *config.Config) {
	cfg.Flags.Color = cfg.Flags.ColorStr == "always" &&
		!terminal.IsPiped() &&
		!terminal.NoColorEnv()

	config.SetVerbosity(cfg.Flags.Verbose)

	// load config from YAML
	if err := config.Load(cfg.Path.ConfigFile); err != nil {
		slog.Error("loading config", "err", err)
	}

	// FIX: remove usage from global `Fzf`.
	cfg.Menu = config.Fzf

	// set menu
	menu.SetConfig(config.Fzf)

	// enable global color
	menu.ColorEnable(cfg.Flags.Color)
	color.Enable(cfg.Flags.Color)

	// terminal interactive mode
	terminal.NonInteractiveMode(cfg.Flags.Force)

	// git config
	git.SetConfig(cfg)
}

// Setup registers all application commands with the CLI.
func Setup(root *cobra.Command) {
	cli.Register(
		create.NewCmd(),
		records.NewCmd(),
		tags.NewCmd(),
		database.NewCmd(),
		gitCmd.NewCmd(),
		io.NewCmd(),
		settings.NewCmd(),
		setup.NewCmd(),
	)

	cli.AttachTo(root)
}

// Execute executes the provided root command and exits on error.
func Execute(r *cobra.Command) {
	if err := r.Execute(); err != nil {
		sys.ErrAndExit(err)
	}
}
