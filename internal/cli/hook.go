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
	"github.com/mateconpizza/gm/internal/ui/formatter"
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

// HookE is the function signature used for Cobra PersistentPreRunE hooks.
// It takes the current command and its arguments, and returns an error
// if the pre-run checks fail.
type HookE func(cmd *cobra.Command, args []string) error

type Hook func(cmd *cobra.Command, args []string)

// ChainHooks chains multiple Hook functions into a single PersistentPreRunE.
// Hooks run in the order provided. The first non-nil error stops execution.
func ChainHooks(hooks ...HookE) HookE {
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
func HookEnsureDatabase(app *application.App) HookE {
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
			return fmt.Errorf("%w: %q", application.ErrDatabaseInvalidName, app.DBName)
		}

		// If check already passed, return early
		if databaseChecked {
			return nil
		}

		if files.Exists(app.Path.DB()) {
			databaseChecked = true
			return nil
		}

		if err := checkDatabaseLocked(app.Path.DB()); err != nil {
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

	if files.Exists(app.Path.DB()) {
		if ok, _ := db.IsInitializedFromPath(cmd.Context(), app.Path.DB()); ok {
			return fmt.Errorf("%w: %q", db.ErrDBExistsAndInit, app.DBName)
		}

		return fmt.Errorf("%q %w", app.DBName, db.ErrDBExists)
	}

	return nil
}

// HookGitEnsureEnv ensures Git environment is properly set up.
func HookGitEnsureEnv(app *application.App) HookE {
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

		isInitialized := git.Initialized(app.Path.Git())
		if !app.GitEnabled() && isInitialized {
			return git.ErrGitDisabled
		}

		if !isInitialized {
			i := ansi.BrightYellow.With(ansi.Italic).Sprint("git init")
			return fmt.Errorf("%w: use %s to setup", git.ErrGitNotInitialized, i)
		}

		return nil
	}
}

// HookGitSync synchronizes Git repository with current database state.
func HookGitSync(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			if arg == "-h" || arg == "--help" || arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		// Walk up the command chain: skip if any ancestor declares skip-git-sync
		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations["skip-git-sync"]; ok && v == "true" {
				slog.Debug("skipping git sync", "command", cmd.CommandPath(), "skipped_by", c.CommandPath())
				return nil
			}
		}

		slog.Debug("hook: git sync, checking for changes")
		ctx, cancel := context.WithTimeout(cmd.Context(), 100*time.Second)
		defer cancel()

		msg := cmd.Short
		if msg == "" {
			msg = cmd.Name() + " hook sync"
		}

		return gitops.Sync(ctx, app, fmt.Sprintf("[%s] %s", app.DBBaseName(), msg))
	}
}

// HookGitPrune checks for differences between database and local repo and
// syncs them.
func HookGitPrune(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			if arg == "-h" || arg == "--help" || arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		slog.Debug("hook: checks for differences between database and local repo and syncs them")
		m, err := gitops.NewManager(app)
		if err != nil {
			return fmt.Errorf("hook git: new git manager: %w", err)
		}

		if !m.IsTracked(app.DBBaseName()) {
			slog.Debug("hook git: repo not tracked", "name", app.DBBaseName())
			return nil
		}

		r, err := db.New(cmd.Context(), app.Path.DB())
		if err != nil {
			return fmt.Errorf("hook git: %w", err)
		}
		defer r.Close()

		return gitops.Prune(cmd.Context(), app, r)
	}
}

// HookGitEnableLogging returns a hook that enables git command logging.
func HookGitEnableLogging(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) {
		app.Git.SetWriter(os.Stdout)
	}
}

// HookInjectApp returns a hook that injects the app into the command context.
func HookInjectApp(app *application.App) HookE {
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

		ctx, err := application.ToContext(ctx, app)
		if err != nil {
			return err
		}
		cmd.SetContext(ctx)

		slog.Debug(
			"hook inject app: injected into context",
			"command", cmd.Name(),
			"args", args,
		)

		return nil
	}
}

// HookFormatter sets and registers the application output formatter from CLI
// flags.
func HookFormatter(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		fm, err := formatter.New(formatter.Format(app.Flags.Output))
		if err != nil {
			return err
		}

		app.UI.Formatter = fm

		return nil
	}
}

func HookNil(cmd *cobra.Command, _ []string) error {
	return nil
}

func HookGitLoggingStatus(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) {
		fmt.Println(app.Git.Logging())
	}
}

func HookGitStatus(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) {
		fmt.Println(app.Git.Status())
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
