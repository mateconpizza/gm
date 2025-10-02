// Package cli provides utilities for building and managing Cobra commands.
package cli

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	// subCommands holds all registered CLI subcommands.
	subCommands []*cobra.Command

	// SkipDBCheckAnnotation is used in subcmds declarations to skip the database
	// existence check.
	SkipDBCheckAnnotation = map[string]string{"skip-db-check": "true"}

	// databaseChecked tracks whether the database check has already been
	// performed in the current process.
	databaseChecked bool = false
)

// Hook is the function signature used for Cobra PersistentPreRunE hooks.
// It takes the current command and its arguments, and returns an error
// if the pre-run checks fail.
type Hook func(cmd *cobra.Command, args []string) error

// Register appends one or more subcommands to the global registry.
func Register(cmd ...*cobra.Command) {
	subCommands = append(subCommands, cmd...)
}

// AttachTo attaches all registered subcommands to the given root command.
func AttachTo(cmd *cobra.Command) {
	cmd.AddCommand(subCommands...)
}

// ChainHooks chains multiple Hook functions into a single PersistentPreRunE.
// Hooks run in the order provided. The first non-nil error stops execution.
func ChainHooks(hooks ...Hook) Hook {
	return func(cmd *cobra.Command, args []string) error {
		for _, h := range hooks {
			if h == nil {
				continue
			}
			if err := h(cmd, args); err != nil {
				return err
			}
		}
		return nil
	}
}

// HookEnsureDatabase ensures that a database exists before executing the command.
func HookEnsureDatabase(cmd *cobra.Command, args []string) error {
	if cmd.HasParent() {
		slog.Debug("assert db exists", "command", cmd.Name(), "parent", cmd.Parent().Name())
	} else {
		slog.Debug("assert db exists", "command", cmd.Name())
	}

	// Skip DB check if explicitly unlocking
	unlockFlag, _ := cmd.Flags().GetBool("unlock")
	if unlockFlag {
		return nil
	}

	// Walk up the command chain: skip if any ancestor declares skip-db-check
	for c := cmd; c != nil; c = c.Parent() {
		if v, ok := c.Annotations["skip-db-check"]; ok && v == "true" {
			slog.Debug("skipping db check for", "command", c.Name())
			return nil
		}
	}

	// If check already passed, return early
	if databaseChecked {
		return nil
	}

	app := config.New()
	if files.Exists(app.DBPath) {
		databaseChecked = true
		return nil
	}

	if err := handler.CheckDBLocked(app.DBPath); err != nil {
		return err
	}

	i := color.BrightYellow(app.Cmd, "init").Italic()
	if app.DBName == config.MainDBName {
		return fmt.Errorf("%w: use '%s' to initialize", db.ErrDBMainNotFound, i)
	}

	return fmt.Errorf("%w %q: use '%s' to initialize", db.ErrDBNotFound, app.DBName, i)
}

func HookHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func HookCheckIfDatabaseInitialized(_ *cobra.Command, _ []string) error {
	app := config.New()
	if files.Exists(app.DBPath) {
		if ok, _ := db.IsInitialized(app.DBPath); ok {
			return fmt.Errorf("%w: %q", db.ErrDBExistsAndInit, app.DBName)
		}

		return fmt.Errorf("%q %w", app.DBName, db.ErrDBExists)
	}

	return nil
}

// PrettyVersion formats version in a pretty way.
func PrettyVersion(appName, version string) string {
	name := color.BrightBlue(appName).Bold().String()
	return fmt.Sprintf("%s v%s %s/%s", name, version, runtime.GOOS, runtime.GOARCH)
}
