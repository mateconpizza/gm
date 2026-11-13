package command

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	files "github.com/mateconpizza/gofiles"
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrDBUnlockFirst = errors.New("unlock database first")
)

const (
	AnnotationSkipDBCheck   = "skip-db-check"
	AnnotationSkipFormatter = "skip-formatter"
	AnnotationSkipGitCheck  = "skip-git-check"
	AnnotationSkipGitSync   = "skip-git-sync"
)

var (
	// SkipDBCheck is used in subcmds declarations to skip the database
	// existence check.
	SkipDBCheck = map[string]string{AnnotationSkipDBCheck: "true"}

	// SkipGitSync is used in subcmds declarations to skip the git commit.
	SkipGitSync = map[string]string{AnnotationSkipGitSync: "true"}

	// SkipGitCheck is used in subcmds declarations to skip the git existence
	// check.
	SkipGitCheck = map[string]string{AnnotationSkipGitCheck: "true"}

	// SkipFormatter is used in subcmds declarations to skip the output format
	// validation.
	SkipFormatter = map[string]string{AnnotationSkipFormatter: "true"}

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
		log := app.Logger().With("hook", "ensure-db", "command", cmd.CommandPath())
		if cmd.HasParent() {
			log = log.With("parent", cmd.Parent().Name())
		}

		if exit := dispatch(cmd, app); exit {
			return nil
		}

		// Walk up the command chain: skip if any ancestor declares skip-db-check
		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations[AnnotationSkipDBCheck]; ok && v == "true" {
				log.Debug("skipping db check", "skipped_by", c.CommandPath())
				return nil
			}
		}

		if app.DBBaseName() == "" {
			return fmt.Errorf("%w: %q", application.ErrDatabaseInvalidName, app.DBName)
		}

		// If check already passed, return early
		if databaseChecked {
			return nil
		}

		path := app.Path.DB()
		if files.Exists(path) {
			log.Info("database found", "path", path)
			databaseChecked = true
			return nil
		}

		if err := checkDatabaseLocked(path); err != nil {
			log.Error("database locked", "path", path)
			return err
		}

		p := ansi.NewPalette(app.Flags.Color)
		i := p.BrightYellow.With(p.Italic).Sprintf("%s init", app.Cmd)
		return fmt.Errorf("%w %q: use '%s' to initialize", db.ErrDBNotFound, strings.TrimSuffix(app.DBName, ".db"), i)
	}
}

// HookCheckIfDatabaseInitialized checks if database file exists and is initialized.
// Returns error if database already exists to prevent accidental re-initialization.
func HookCheckIfDatabaseInitialized(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if err := ctx.Err(); err != nil {
		return err
	}

	app, err := application.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if files.Exists(app.Path.DB()) {
		if ok, _ := dbops.IsInitializedFromPath(ctx, app.Path.DB()); ok {
			return fmt.Errorf("%w: %q", db.ErrDBAlreadyInitialized, app.DBName)
		}
		return fmt.Errorf("%q %w", app.DBName, db.ErrDBAlreadyInitialized)
	}

	return nil
}

// HookGitEnsureEnv ensures Git environment is properly set up.
func HookGitEnsureEnv(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		log := app.Logger().With("hook", "git-ensure-env", "command", cmd.CommandPath())

		// This will handle when the `c.DisableFlagParsing` is true and show help command.
		for _, arg := range args {
			if arg == "-h" || arg == "--help" || arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations[AnnotationSkipGitCheck]; ok && v == "true" {
				log.Debug("skipping git environment check", "skipped_by", c.CommandPath())
				return nil
			}
		}

		log.Debug("checking git environment")

		_, err := git.New(app.Path.Git())
		if err != nil {
			return fmt.Errorf("hook git: %w", err)
		}

		if cmd.Name() == "init" {
			log.Debug("git setup command, skipping initialization check")
			return nil
		}

		isInitialized := git.Initialized(app.Path.Git())
		if !app.GitEnabled() && isInitialized {
			log.Debug("git is disabled but repository is initialized")
			return git.ErrGitDisabled
		}

		if !isInitialized {
			log.Debug("git repository not initialized")
			p := ansi.NewPalette(app.Flags.Color)
			i := p.BrightYellow.With(p.Italic).Sprintf("%s git init", app.Cmd)
			return fmt.Errorf("%w: use '%s' to setup", git.ErrGitNotInitialized, i)
		}

		return nil
	}
}

// HookGitSync synchronizes Git repository with current database state.
func HookGitSync(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := ctx.Err(); err != nil {
			return err
		}

		for _, arg := range args {
			if arg == "-h" || arg == "--help" || arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		log := app.Logger().With("hook", "git-sync", "command", cmd.CommandPath())

		// Walk up the command chain: skip if any ancestor declares skip-git-sync
		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations[AnnotationSkipGitSync]; ok && v == "true" {
				log.Debug("skipping git sync", "skipped_by", c.CommandPath())
				return nil
			}
		}

		log.Debug("checking for changes")
		ctx, cancel := context.WithTimeout(ctx, 100*time.Second)
		defer cancel()

		msg := cmd.Short
		if msg == "" {
			msg = cmd.Name()
		}

		app.Git.SetWriter(os.Stdout)
		gm := app.Git.Manager()

		return gitops.Sync(ctx,
			app,
			gm,
			gm.NewRepo(app.DBBaseName()).CommitMsg(git.RepoAction(msg), ""),
		)
	}
}

// HookGitPrune checks for differences between database and local repo and
// syncs them.
func HookGitPrune(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if err := ctx.Err(); err != nil {
			return err
		}

		for _, arg := range args {
			if arg == "-h" || arg == "--help" || arg == "help" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		app.Logger().Debug("hook: checks for differences between database and local repo and syncs them")
		app.Git.SetWriter(os.Stdout)
		gm, err := gitops.NewManager(app)
		if err != nil {
			return fmt.Errorf("hook git: new git manager: %w", err)
		}

		if !gm.IsTracked(app.DBBaseName()) {
			app.Logger().Debug("hook git: repo not tracked", "name", app.DBBaseName())
			return nil
		}

		r, err := db.New(ctx, app.Path.DB())
		if err != nil {
			return fmt.Errorf("hook git: %w", err)
		}
		defer r.Close()

		sess := app.Session().
			WithConfirmation(true).
			WithValidation(true)

		gr := gitops.NewRepo(gm, r.BaseName(), sess, app.Flags.Color)

		bs, err := r.All(ctx)
		if err != nil {
			return err
		}

		gitops.RegisterHooks(r, app, gr)

		return gitops.PruneRepo(ctx, gm, gr, bs)
	}
}

func HookGitEnsureTracked(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		gm := app.Git.Manager()
		if gm.IsTracked(app.DBBaseName()) {
			return nil
		}

		return fmt.Errorf("%w: %q", git.ErrGitNotTracked, app.DBBaseName())
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
		log := app.Logger().With("hook", "inject-app", "command", cmd.CommandPath())

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
			log.Debug("context was nil, using background")
		}

		if _, err := application.FromContext(ctx); err == nil {
			log.Debug("app already in context, skipping")
			return nil
		}

		ctx, err := application.ToContext(ctx, app)
		if err != nil {
			return err
		}
		cmd.SetContext(ctx)

		log.Debug("app injected into context", "args", args)

		return nil
	}
}

// HookFormatter sets and registers the application output formatter from CLI
// flags.
func HookFormatter(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		log := app.Logger().With("hook", "formatter", "command", cmd.CommandPath())

		for c := cmd; c != nil; c = c.Parent() {
			if v, ok := c.Annotations[AnnotationSkipFormatter]; ok && v == "true" {
				log.Debug("skipping UI formatter set", "skipped_by", c.CommandPath())
				return nil
			}
		}

		isOutputFlagSet := cmd.Flags().Changed("output")
		if app.Flags.Menu && isOutputFlagSet {
			app.Menu.Format = app.Flags.Output
		}

		fm, err := formatter.New(formatter.Format(app.Menu.Format))
		if err != nil {
			return err
		}

		app.WithFormatter(fm)
		log.Debug("formatter set and registered", "format", app.Menu.Format)

		return nil
	}
}

func HookValidateFlags(flag *string, supported []string) HookE {
	return func(cmd *cobra.Command, args []string) error {
		if !slices.Contains(supported, *flag) {
			return fmt.Errorf("%w %q; valid options: %s",
				ErrInvalidOption,
				*flag,
				strings.Join(supported, ", "))
		}
		return nil
	}
}

func HookEnsureAppInitialize(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		if exit := dispatch(cmd, app); exit {
			return nil
		}

		p := ansi.NewPalette(app.Flags.Color)
		command := p.BrightYellow.With(p.Italic).
			Sprintf("%s init", app.Cmd)

		mainDBPath := filepath.Join(app.Path.Data, application.MainDBName)
		if files.Exists(app.Path.Home()) && !files.Exists(mainDBPath) {
			return fmt.Errorf("%w: run '%s' for setup", application.ErrMainNotFound, command)
		}

		if files.Exists(app.Path.Home()) {
			return nil
		}

		return fmt.Errorf("%w: run '%s' for setup", application.ErrAppNoSetup, command)
	}
}

// HookGitManager returns a hook that initializes and attaches the git manager
// when git is enabled.
func HookGitManager(app *application.App) HookE {
	return func(cmd *cobra.Command, args []string) error {
		log := app.Logger().With("hook", "git-manager", "command", cmd.CommandPath())

		if !app.Git.Enabled {
			log.Debug("git is disabled, skipping git manager initialization")
			return nil
		}

		gm, err := gitops.NewManager(app)
		if err != nil {
			return err
		}

		app.Git.SetManager(gm)
		log.Debug("git manager initialized and attached")

		return nil
	}
}

func HookGitLoggingStatus(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stdout, app.Git.Logging())
	}
}

func HookGitStatus(app *application.App) Hook {
	return func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stdout, app.Git.Status())
	}
}

// checkDatabaseLocked checks if the database is locked.
func checkDatabaseLocked(p string) error {
	err := locker.IsLocked(p)
	if err != nil {
		if errors.Is(err, locker.ErrFileLocked) {
			return ErrDBUnlockFirst
		}

		return fmt.Errorf("%w", err)
	}

	return nil
}

func dispatch(cmd *cobra.Command, app *application.App) bool {
	log := app.Logger().With("command", cmd.CommandPath())

	// Skip Home initialized check
	if cmd.CommandPath() == app.Cmd+" init" {
		log.Debug("skipping initialized check")
		return true
	}

	// Skip DB check if explicitly unlocking
	if unlock, _ := cmd.Flags().GetBool("unlock"); unlock {
		log.Debug("ensuring database dispatch", "unlock", unlock)
		return true
	}

	return false
}
