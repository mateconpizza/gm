// Package cmd contains the core commands and initialization logic for the
// application.
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/git"
)

// NewRootCmd is the main command.
func NewRootCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:               app.Cmd + " [query]",
		Args:              cobra.MinimumNArgs(0),
		SilenceUsage:      true,
		PersistentPreRunE: cli.ChainHooks(cli.HookInjectApp(app), cli.HookEnsureDatabase(app)),
		RunE:              rootCmdFunc(app),
	}

	registerRootFlags(c, app)
	setupRootCmd(c)

	// Initialize application state before command execution
	cobra.OnInitialize(func() {
		initAppConfig(app)

		app.Initialize()

		// set up git env
		remote, _ := git.Remote(c.Context(), app.Path.Git())
		app.Git.Remote = remote
	})

	// Register cleanup hooks to be executed on shutdown/exit
	registerCleanups(app)

	return c
}

// Execute executes the provided root command and exits on error.
func Execute(c *cobra.Command) error {
	ctx, stop := sys.WithSignalContext(context.Background())
	defer stop()

	return c.ExecuteContext(ctx)
}

func rootCmdFunc(app *application.App) cli.HookE {
	return func(cmd *cobra.Command, args []string) error {
		if app.Flags.Version {
			fmt.Print(app.PrettyVersion())
			return nil
		}

		fm, err := formatter.New(formatter.Format(app.Flags.Output))
		if err != nil {
			return err
		}

		m := picker.NewMainMenu(app, fm)
		a := func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
			t, f := d.Console(), app.Flags

			switch {
			case app.Flags.Field != "":
				return printer.ByField(ctx, t, f.Field, bs) // TODO: experimental
			case app.Flags.Preview != "":
				return printer.MenuPreview(t, bs, f.Preview)
			case app.Flags.Output != "":
				return printer.Display(ctx, t, f.Output, bs)
			default:
				return printer.Records(ctx, t, bs)
			}
		}

		return cmdutil.Execute(cmd, args, m, a)
	}
}

func setupRootCmd(c *cobra.Command) {
	// Add custom template function used inside usage/help templates
	cobra.AddTemplateFunc("hasFlags", cmdutil.HasFlags)

	// Override default usage template with a custom one
	c.SetUsageTemplate(cmdutil.UsageTemplate)

	// Keep flag order as defined (do not sort alphabetically)
	c.PersistentFlags().SortFlags = false

	// Hide the default completion command from help output
	c.CompletionOptions.HiddenDefaultCmd = true

	// Suppress automatic error printing (handled manually elsewhere)
	c.SilenceErrors = true

	// Disable command suggestions on invalid input
	c.DisableSuggestions = true

	// Minimum edit distance for suggestions (irrelevant if disabled, but explicit)
	c.SuggestionsMinimumDistance = 1

	// Remove the default help command from the command tree
	c.SetHelpCommand(&cobra.Command{Hidden: true})

	// Preserve command registration order (no automatic sorting)
	cobra.EnableCommandSorting = false

	// Ensure PersistentPreRun hooks are executed across command traversal
	cobra.EnableTraverseRunHooks = true
}
