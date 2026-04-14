// Package cmd contains the core commands and initialization logic for the
// application.
package cmd

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/add"
	"github.com/mateconpizza/gm/cmd/appcfg"
	"github.com/mateconpizza/gm/cmd/archive"
	"github.com/mateconpizza/gm/cmd/check"
	"github.com/mateconpizza/gm/cmd/clean"
	"github.com/mateconpizza/gm/cmd/cmdutil"
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
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/cleanup"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// TODO: let the user set the default database.
// - [ ] gm db use <name> (this will set it as default?)

// NewRootCmd is the main command.
func NewRootCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:               cfg.Cmd + " [query]",
		Args:              cobra.MinimumNArgs(0),
		SilenceUsage:      true,
		PersistentPreRunE: cli.ChainHooks(cli.HookInjectConfig(cfg), cli.HookEnsureDatabase),
		Version:           cli.PrettyVersion(cfg.Name, cfg.Info.Version),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuMainForRecords[bookmark.Bookmark](cfg)
			return cmdutil.Execute(cmd, args, m, handler.Display)
		},
	}

	cobra.AddTemplateFunc("hasFlags", cmdutil.HasFlags)

	c.SetUsageTemplate(cmdutil.UsageTemplate)
	c.PersistentFlags().SortFlags = false

	// local
	cmdutil.FlagFormat(c, cfg)
	cmdutil.FlagsFilter(c, cfg)
	cmdutil.FlagMenu(c, cfg)

	// global
	g := c.PersistentFlags()
	g.StringVar(&cfg.DBName, "db", config.MainDBName, "database name")
	g.StringVar(&cfg.Flags.ColorStr, "color", "always", "output with colors [always|never]")
	g.BoolVar(&cfg.Flags.Force, "force", false, "force action")
	g.BoolVarP(&cfg.Flags.Yes, "yes", "y", false, "assume yes")
	g.CountVarP(&cfg.Flags.Verbose, "verbose", "v", "increase verbosity (-v, -vv, -vvv)")

	cobra.OnInitialize(func() {
		cfg.Initialize()
		initAppConfig(c.Context(), cfg)
	})

	// cmd settings
	c.CompletionOptions.HiddenDefaultCmd = true
	c.SilenceErrors = true
	c.DisableSuggestions = true
	c.SuggestionsMinimumDistance = 1
	c.SetHelpCommand(&cobra.Command{Hidden: true})
	cobra.EnableCommandSorting = false
	cobra.EnableTraverseRunHooks = true

	registerCleanups(cfg)

	return c
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
		edit.NewCmd(cfg),
		rm.NewCmd(cfg),
		open.NewCmd(cfg),
		yank.NewCmd(cfg),
		notes.NewCmd(cfg),
		qrcmd.NewCmd(cfg),
		check.NewCmd(cfg),
		tag.NewCmd(cfg),
		clean.NewCmd(cfg),
		archive.NewCmd(cfg),
		gitCmd.NewCmd(cfg),
		newAdminCmd(cfg),
		setup.NewCmd(),
	)
}

// Execute executes the provided root command and exits on error.
func Execute(r *cobra.Command) error {
	ctx, stop := sys.WithSignalContext(context.Background())
	defer stop()

	return r.ExecuteContext(ctx)
}

func newAdminCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "admin",
		Short: "manage database and config",
	}

	c.AddCommand(
		database.NewCmd(cfg),
		appcfg.NewCmd(cfg),
		in.NewCmd(cfg),
		out.NewCmd(cfg),
	)

	c.Flags().BoolVarP(&cfg.Flags.Help, "help", "h", false, "")
	_ = c.Flags().MarkHidden("help")

	return c
}

func registerCleanups(_ *config.Config) {
	// close all open connections
	cleanup.Register(func() error {
		db.Shutdown()
		return nil
	})
}
