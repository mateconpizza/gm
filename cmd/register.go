package cmd

import (
	"context"
	"log/slog"
	"strings"

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
	"github.com/mateconpizza/gm/internal/ui/formatter"
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
	local := c.Flags()
	local.SortFlags = false

	global := c.PersistentFlags()
	global.SortFlags = false

	// Register common filtering flags (tags, head, tail)
	global.StringSliceVarP(&app.Flags.Tags, "tag", "t", nil, "filter by tag(s)")

	// Limit results (head/tail semantics)
	global.IntVarP(&app.Flags.Head, "head", "H", 0, "limit to first N bookmarks")
	global.IntVarP(&app.Flags.Tail, "tail", "T", 0, "limit to last N bookmarks")

	// Interactive mode (e.g. TUI or prompt-based selection)
	global.BoolVarP(&app.Flags.Menu, "menu", "m", false, "select interactively")

	// Output formatting (e.g. json, csv, table, etc.)
	cmdutil.FlagOutput(c, app, formatter.ValidFormats())

	// Field selection for output projection
	fields := []string{"id", "url", "title", "tags", "desc"}
	global.StringVarP(&app.Flags.Field, "fields", "f", "", "select fields: "+strings.Join(fields, ", "))

	// Verbosity level
	global.CountVarP(&app.Flags.Verbose, "verbose", "v", "increase verbosity (-v, -vv, -vvv)")

	// Non-interactive confirmation
	global.BoolVarP(&app.Flags.Yes, "yes", "y", false, "assume yes")

	// Output colorization policy
	global.StringVar(&app.Flags.ColorStr, "color", "always", "colorize output: always, never")

	// Force execution even if safeguards would prevent it
	global.BoolVar(&app.Flags.Force, "force", false, "force action")

	// Database selection
	global.StringVar(&app.DBName, "db", application.MainDBName, "database name")

	// Override default help flag to hide it
	global.Bool("help", false, "")
	_ = global.MarkHidden("help")

	// Hidden/internal flag
	global.StringVar(&app.Flags.Preview, "preview", "", "")
	_ = global.MarkHidden("preview")

	// Version flag
	local.BoolVarP(&app.Flags.Version, "version", "V", false, "version for "+app.Cmd)
}
