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
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/color"
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
		slog.Debug("assert db exists", "parent", cmd.Parent().Name())
	}
	slog.Debug("assert db exists", "command", cmd.Name())

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

	if err := checkDatabaseLocked(app.DBPath); err != nil {
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

// HookCheckIfDatabaseInitialized checks if database file exists and is initialized.
// Returns error if database already exists to prevent accidental re-initialization.
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

// HookEnsureGitEnv ensures Git environment is properly set up.
// Shows help if help flags are present, verifies Git is installed,
// and checks if repository is initialized (except for init/import commands).
func HookEnsureGitEnv(c *cobra.Command, args []string) error {
	// This will handle when the `c.DisableFlagParsing` is true and show help command.
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			_ = c.Help()
			os.Exit(0)
		}
	}

	gitCmd, err := sys.Which("git")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	app := config.New()
	gm := git.NewGit(app.Git.Path, git.WithCmd(gitCmd))

	switch c.Name() {
	case "init", "import":
		return nil
	}

	if !gm.IsInitialized() {
		return git.ErrGitNotInitialized
	}

	return nil
}

// HookGitSync synchronizes Git repository with current database state.
func HookGitSync(c *cobra.Command, args []string) error {
	app := config.New()

	if !git.IsInitialized(app.Git.Path) {
		return nil
	}

	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}

	if !gr.IsTracked() {
		return nil
	}

	r, err := db.New(app.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bs, err := r.All(context.Background())
	if err != nil {
		return err
	}

	updated, err := gr.Write(bs)
	if err != nil {
		return err
	}

	if !updated {
		return nil
	}

	return gr.Commit(c.Short)
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
