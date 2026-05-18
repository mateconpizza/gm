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
)

// NewRootCmd is the main command.
func NewRootCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                app.Cmd + " [query]",
		Args:               cobra.MinimumNArgs(0),
		SilenceUsage:       true,
		PersistentPreRunE:  cli.ChainHooks(cli.HookInjectApp(app), cli.HookEnsureDatabase(app)),
		PersistentPostRunE: cli.HookGitSync,
		RunE:               rootCmdFunc(app),
	}

	registerRootFlags(c, app)
	setupRootCmd(c)

	// Initialize application state before command execution
	cobra.OnInitialize(func() {
		initAppConfig(c.Context(), app)

		app.Initialize()
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

func rootCmdFunc(app *application.App) cli.Hook {
	return func(cmd *cobra.Command, args []string) error {
		if app.Flags.Version {
			fmt.Print(app.PrettyVersion())
			return nil
		}

		fm, err := formatter.New(app.Flags.Output)
		if err != nil {
			return err
		}
		m := picker.NewMainMenu(app, fm)

		return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
			t, f := d.Console(), app.Flags

			switch {
			case app.Flags.Field != "":
				return printer.ByField(d.Context(), t, f.Field, bs) // TODO: experimental
			case app.Flags.Preview != "":
				return printer.MenuPreview(t, bs, f.Preview)
			case app.Flags.Output != "":
				return printer.Display(t, f.Output, bs)
			default:
				return printer.Records(d.Context(), t, bs)
			}
		})
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
