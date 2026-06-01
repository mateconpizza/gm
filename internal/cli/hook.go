// Package cli provides utilities for building and managing Cobra commands.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
	"github.com/mateconpizza/gm/pkg/git"
)

var (
	// SkipDBCheck is used in subcmds declarations to skip the database
	// existence check.
	SkipDBCheck = map[string]string{"skip-db-check": "true"}

	// SkipGitSync is used in subcmds declarations to skip the git commit.
	SkipGitSync = map[string]string{"skip-git-sync": "true"}

	// SkipGitCheck is used in subcmds declarations to skip the git existence
	// check.
	SkipGitCheck = map[string]string{"skip-git-check": "true"}

	// databaseChecked tracks whether the database check has already been
	// performed in the current process.
	databaseChecked bool = false
)

// ChainAnnotations merges multiple annotation maps into one.
func ChainAnnotations(annotations ...map[string]string) map[string]string {
	m := make(map[string]string)
	for _, ann := range annotations {
		maps.Copy(m, ann)
	}
	return m
}

// Hook is the function signature used for Cobra PersistentPreRunE hooks.
// It takes the current command and its arguments, and returns an error
// if the pre-run checks fail.
type Hook func(cmd *cobra.Command, args []string) error

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

// HookEnsureDatabase ensures the database exists before command execution.
// Skips check for unlock operations and commands annotated with "skip-db-check".
// Returns an error if database is missing, locked, or needs initialization.
func HookEnsureDatabase(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) error {
		if cmd.HasParent() {
			slog.Debug("ensure database", "parent", cmd.Parent().Name())
		}
		slog.Debug("ensure database", "command", cmd.Name())

		if exit := dispatch(cmd); exit {
			return nil
		}

		// Walk up the command chain: skip if any ancestor declares skip-db-check
		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations["skip-db-check"]; ok && v == "true" {
				slog.Debug("skipping db check for", "command", c.Name())
				return nil
			}
		}

		if files.StripSuffixes(app.DBName) == "" {
			return fmt.Errorf("::%w: %q", application.ErrDatabaseInvalidName, app.DBName)
		}

		// If check already passed, return early
		if databaseChecked {
			return nil
		}

		if files.Exists(app.Path.Database) {
			databaseChecked = true
			return nil
		}

		if err := checkDatabaseLocked(app.Path.Database); err != nil {
			return err
		}

		i := ansi.BrightYellow.With(ansi.Italic).Sprintf("%s init", app.Cmd)
		return fmt.Errorf("%w %q: use %s to initialize", db.ErrDBNotFound, strings.TrimSuffix(app.DBName, ".db"), i)
	}
}

func HookHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

// HookCheckIfDatabaseInitialized checks if database file exists and is initialized.
// Returns error if database already exists to prevent accidental re-initialization.
func HookCheckIfDatabaseInitialized(cmd *cobra.Command, _ []string) error {
	app, err := application.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if files.Exists(app.Path.Database) {
		if ok, _ := db.IsInitializedFromPath(cmd.Context(), app.Path.Database); ok {
			return fmt.Errorf("%w: %q", db.ErrDBExistsAndInit, app.DBName)
		}

		return fmt.Errorf("%q %w", app.DBName, db.ErrDBExists)
	}

	return nil
}

// HookEnsureGitEnv ensures Git environment is properly set up.
// Shows help if help flags are present, verifies Git is installed,
// and checks if repository is initialized (except for init/import/clone commands).
func HookEnsureGitEnv(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) error {
		// This will handle when the `c.DisableFlagParsing` is true and show help command.
		for _, arg := range args {
			if arg == "-h" || arg == "--help" || arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations["skip-git-check"]; ok && v == "true" {
				slog.Debug("skipping git sync for", "command", c.Name())
				return nil
			}
		}

		_, err := git.New(app.Path.Git())
		if err != nil {
			return fmt.Errorf("hook git: %w", err)
		}

		switch cmd.Name() {
		case "init", "import", "clone":
			return nil
		}

		if !app.Git.Enabled {
			i := ansi.BrightYellow.With(ansi.Italic).Sprint("git init")
			return fmt.Errorf("%w: use %s to setup", git.ErrGitNotInitialized, i)
		}

		return nil
	}
}

// HookGitSync synchronizes Git repository with current database state.
func HookGitSync(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) error {
		// Walk up the command chain: skip if any ancestor declares skip-git-sync
		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations["skip-git-sync"]; ok && v == "true" {
				slog.Debug("skipping git sync for", "command", c.Name())
				return nil
			}
		}

		slog.Debug("hook: git sync, checking for changes")
		app, err := application.FromContext(cmd.Context())
		if err != nil {
			return fmt.Errorf("hook-git: failed to get config: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()

		msg := cmd.Short
		if msg == "" {
			msg = cmd.Name() + " hook sync"
		}

		return gitops.Sync(ctx, app, fmt.Sprintf("[%s] %s", app.DBBaseName(), msg))
	}
}

// HookInjectApp returns a hook that injects the app into the command context.
func HookInjectApp(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if ctx == nil {
			ctx = context.Background()
			slog.Debug("hook inject app: context was nil, using background")
		}

		if _, err := application.FromContext(ctx); err == nil {
			// App already injected, skip
			slog.Debug(
				"hook inject app: already present, skipping",
				"command", cmd.Name(),
			)
			return nil
		}

		cmd.SetContext(application.ToContext(ctx, app))

		slog.Debug(
			"hook inject app: injected into context",
			"command", cmd.Name(),
			"args", args,
		)

		return nil
	}
}

// checkDatabaseLocked checks if the database is locked.
func checkDatabaseLocked(p string) error {
	err := locker.IsLocked(p)
	if err != nil {
		if errors.Is(err, locker.ErrFileLocked) {
			return db.ErrDBUnlockFirst
		}

		return fmt.Errorf("%w", err)
	}

	return nil
}

func dispatch(cmd *cobra.Command) bool {
	// Skip DB check if explicitly unlocking
	if unlock, _ := cmd.Flags().GetBool("unlock"); unlock {
		slog.Debug("ensure database dispatch", "unlock", unlock)
		return true
	}

	return false
}
