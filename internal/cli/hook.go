package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

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
func HookEnsureDatabase(cmd *cobra.Command, args []string) error {
	if cmd.HasParent() {
		slog.Debug("ensure database", "parent", cmd.Parent().Name())
	}
	slog.Debug("ensure database", "command", cmd.Name())

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

	cfg, err := Config(cmd)
	if err != nil {
		return err
	}

	if files.Exists(cfg.DBPath) {
		databaseChecked = true
		return nil
	}

	if err := checkDatabaseLocked(cfg.DBPath); err != nil {
		return err
	}

	i := ansi.BrightYellow.With(ansi.Italic).Sprintf("%s init", cfg.Cmd)
	if cfg.DBName == config.MainDBName {
		return fmt.Errorf("%w: use %s to initialize", db.ErrDBMainNotFound, i)
	}

	return fmt.Errorf("%w %q: use %s to initialize", db.ErrDBNotFound, cfg.DBName, i)
}

func HookHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

// HookCheckIfDatabaseInitialized checks if database file exists and is initialized.
// Returns error if database already exists to prevent accidental re-initialization.
func HookCheckIfDatabaseInitialized(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if files.Exists(cfg.DBPath) {
		if ok, _ := db.IsInitializedFromPath(cmd.Context(), cfg.DBPath); ok {
			return fmt.Errorf("%w: %q", db.ErrDBExistsAndInit, cfg.DBName)
		}

		return fmt.Errorf("%q %w", cfg.DBName, db.ErrDBExists)
	}

	return nil
}

// HookEnsureGitEnv ensures Git environment is properly set up.
// Shows help if help flags are present, verifies Git is installed,
// and checks if repository is initialized (except for init/import commands).
func HookEnsureGitEnv(cmd *cobra.Command, args []string) error {
	// This will handle when the `c.DisableFlagParsing` is true and show help command.
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			_ = cmd.Help()
			os.Exit(0)
		}
	}

	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	_, err = git.NewManager(cmd.Context(), cfg.Git.Path)
	if err != nil {
		return fmt.Errorf("hook git: %w", err)
	}

	switch cmd.Name() {
	case "init", "import", "clone":
		return nil
	}

	if !cfg.Git.Enabled {
		return git.ErrGitNotInitialized
	}

	return nil
}

// HookGitSync synchronizes Git repository with current database state.
func HookGitSync(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if !cfg.Git.Enabled {
		return nil
	}

	gr, err := git.NewRepo(cfg.DBPath)
	if err != nil {
		return err
	}

	if !gr.IsTracked() {
		return nil
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	bs, err := r.All(cmd.Context())
	if err != nil {
		return err
	}

	updated, err := gr.Write(bs, cfg.Flags.Force)
	if err != nil {
		return err
	}

	if !updated {
		return nil
	}

	return gr.Commit(cmd.Short)
}

// HookInjectConfig returns a hook that injects the config into the command context.
func HookInjectConfig(cfg *config.Config) Hook {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// If context is nil, create a new one
		if ctx == nil {
			ctx = context.Background()
		}

		// Check if config already exists
		if _, err := config.FromContext(ctx); err == nil {
			// Config already injected, skip
			return nil
		}

		// Inject config into context
		cmd.SetContext(config.ToContext(ctx, cfg))
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
