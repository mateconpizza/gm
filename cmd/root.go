// Package cmd contains the core commands and initialization logic for the
// application.
package cmd

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/add"
	"github.com/mateconpizza/gm/cmd/appcfg"
	"github.com/mateconpizza/gm/cmd/database"
	gitCmd "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/cmd/io"
	"github.com/mateconpizza/gm/cmd/records"
	"github.com/mateconpizza/gm/cmd/setup"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

const usageTemplate = `usage: {{if .Runnable}}{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}} [command]{{end}}{{if gt (len .Aliases) 0}}

aliases: {{.NameAndAliases}}{{end}}
{{ if gt (len .Commands) 0}}
commands:
{{range .Commands}}{{if or .IsAvailableCommand (eq .Name "help")}}  {{rpad .Name .NamePadding}} {{.Short}}
{{end}}{{end}}{{end}}
flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
`

// NewRootCmd is the main command.
func NewRootCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:               cfg.Cmd + " [query]",
		Args:              cobra.MinimumNArgs(0),
		SilenceUsage:      true,
		PersistentPreRunE: cli.ChainHooks(cli.HookInjectConfig(cfg), cli.HookEnsureDatabase),
		RunE:              records.Cmd,
		Version:           cli.PrettyVersion(cfg.Name, cfg.Info.Version),
	}

	cmd.SetUsageTemplate(usageTemplate)
	cmd.PersistentFlags().SortFlags = false

	// Global flags
	cmd.PersistentFlags().StringVarP(&cfg.DBName, "name", "n", config.MainDBName,
		"database name")
	cmd.PersistentFlags().StringVar(&cfg.Flags.ColorStr, "color", "always",
		"output with pretty colors [always|never]")
	cmd.PersistentFlags().BoolVar(&cfg.Flags.Force, "force", false,
		"force action")
	cmd.PersistentFlags().BoolVarP(&cfg.Flags.Yes, "yes", "y", false,
		"assume \"yes\" on most questions")
	cmd.PersistentFlags().CountVarP(&cfg.Flags.Verbose, "verbose", "v",
		"increase verbosity (-v, -vv, -vvv)")

	// Initialize flags for records commands
	records.InitFlags(cmd, cfg)

	cobra.OnInitialize(func() {
		cfg.Initialize()
		initAppConfig(cmd.Context(), cfg)
	})

	// cmd settings
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.SilenceErrors = true
	cmd.DisableSuggestions = true
	cmd.SuggestionsMinimumDistance = 1
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cobra.EnableCommandSorting = false
	cobra.EnableTraverseRunHooks = true

	registerCleanups(cfg)

	return cmd
}

func initAppConfig(ctx context.Context, cfg *config.Config) {
	cfg.Flags.Color = cfg.Flags.ColorStr == "always" &&
		!terminal.IsPiped() &&
		!terminal.NoColorEnv()

	config.SetVerbosity(cfg.Flags.Verbose)

	// load config from YAML
	if err := config.Load(cfg); err != nil {
		slog.Error("loading config", "err", err)
	}

	// enable global color
	if !cfg.Flags.Color {
		ansi.DisableColor()
		frame.DisableColor()
	}

	// terminal interactive mode
	terminal.NonInteractiveMode(cfg.Flags.Yes)

	// git config
	git.SetConfig(ctx, cfg)
}

// Setup registers all application commands with the CLI.
func Setup(root *cobra.Command, cfg *config.Config) {
	root.AddCommand(
		add.NewCmd(cfg),
		records.NewCmd(cfg),
		database.NewCmd(cfg),
		gitCmd.NewCmd(cfg),
		io.NewCmd(cfg),
		appcfg.NewCmd(cfg),
		setup.NewCmd(),
	)
}

// Execute executes the provided root command and exits on error.
func Execute(r *cobra.Command) error {
	ctx, stop := sys.WithSignalContext(context.Background())
	defer stop()

	return r.ExecuteContext(ctx)
}

func registerCleanups(_ *config.Config) {
	// sync git
	// cleanup.Register(func() {
	// FIX: implement
	// 	if err := git.Sync(cfg); err != nil {
	// 		slog.Error("GitSync", slog.String("error", err.Error()))
	// 	}
	// })

	// close all open connections
	cleanup.Register(db.Shutdown)
}
