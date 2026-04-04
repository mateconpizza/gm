package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/files"
)

// NewCmd is the git command.
func NewCmd(cfg *config.Config) *cobra.Command {
	gitCmd := &cobra.Command{
		Use:                "git",
		Short:              "Git commands",
		Aliases:            []string{"g"},
		PersistentPreRunE:  cli.HookEnsureGitEnv,
		RunE:               gitCommandFunc,
		DisableFlagParsing: true,
	}

	// git tracker
	gitTrackerCmd.Flags().SortFlags = false
	gitTrackerCmd.Flags().BoolVarP(&cfg.Flags.List, "list", "l", false,
		"status tracked databases")
	gitTrackerCmd.Flags().BoolVarP(&cfg.Flags.Track, "track", "t", false,
		"track database in git")
	gitTrackerCmd.Flags().BoolVarP(&cfg.Flags.Untrack, "untrack", "u", false,
		"untrack database in git")
	gitCmd.AddCommand(gitTrackerCmd)

	// git initializer
	initCmd.Flags().BoolVar(&cfg.Flags.Redo, "redo", false,
		"reinitialize")
	gitCmd.AddCommand(initCmd)

	// git import from repo
	ImportCmd.Flags().StringVarP(&cfg.Flags.Path, "uri", "i", "",
		"repo URI to import")
	gitCmd.AddCommand(ImportCmd) // public

	// git clone
	cloneCmd.Flags().BoolVar(&cfg.Flags.Force, "force", false,
		"force clone")
	gitCmd.AddCommand(cloneCmd)

	gitCmd.AddCommand(commitCmd, pushCmd, rawCmd)

	return gitCmd
}

var (
	// rawCmd proxies raw Git commands directly to the underlying git binary.
	rawCmd = &cobra.Command{
		Use:                "raw",
		Short:              "raw git commands",
		DisableFlagParsing: true,
		RunE:               gitCommandFunc,
	}

	// initCmd initializes a new, empty Git repository.
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "create empty Git repository",
		RunE:  initFunc,
	}

	// commitCmd records staged changes in the repository.
	commitCmd = &cobra.Command{
		Use:   "commit",
		Short: "commit changes to the repository",
		RunE:  cli.HookGitSync,
	}

	// ImportCmd clones a Git repository and imports bookmarks.
	ImportCmd = &cobra.Command{
		Use:   "import",
		Short: "import bookmarks from git",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}
			if cfg.Flags.Path == "" {
				return git.ErrGitRepoURLEmpty
			}
			a := app.New(cmd.Context(), app.WithConfig(cfg))

			return importFromClone(a, cmd.Short)
		},
	}

	// pushCmd pushes local changes to the remote repository.
	pushCmd = &cobra.Command{
		Use:                "push",
		Short:              "push changes to the repository",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.FromContext(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get config: %w", err)
			}
			return pushFunc(cmd.Context(), cfg)
		},
	}

	cloneCmd = &cobra.Command{
		Use:   "clone",
		Short: "clone a remote repository",
		RunE:  cloneFunc,
	}
)

// importFromClone clones a git repo and imports its bookmarks.
func importFromClone(a *app.Context, commitMesg string) error {
	cfg := a.Cfg
	tmpPath := filepath.Join(os.TempDir(), cfg.Name+"-clone")
	if files.Exists(tmpPath) {
		_ = files.RemoveAll(tmpPath)
	}
	defer func() { _ = files.RemoveAll(tmpPath) }()

	a.SetConsole(ui.NewDefaultConsole(a.Context(), func(err error) {
		fmt.Println("cleaning up temp files...")
		if err := files.RemoveAll(tmpPath); err != nil {
			slog.Error("cleaning up temp dir", "path", tmpPath)
		}
		sys.ErrAndExit(err)
	}))

	gm, err := git.NewManager(a.Context(), tmpPath)
	if err != nil {
		return err
	}

	imported, err := git.Import(a, gm)
	if err != nil {
		return err
	}
	if !cfg.Git.Enabled {
		slog.Warn("git import: repo not initialized", "path", cfg.Git.Path)
		return nil
	}

	return processImported(a.Console(), imported, commitMesg)
}

func processImported(c *ui.Console, imported []string, commitMesg string) error {
	for _, dbPath := range imported {
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return err
		}

		if gr.IsTracked() {
			if err := gr.Export(); err != nil {
				return err
			}
			if err := gr.Commit(commitMesg); err != nil {
				return err
			}
			continue
		}

		fmt.Println()
		if err := track(c, gr); err != nil {
			return err
		}
	}

	return nil
}

// initFunc creates a new Git repository.
func initFunc(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	gr, err := git.NewRepo(cfg.DBPath)
	if err != nil {
		return err
	}

	if err := gr.Git.Init(cfg.Flags.Redo); err != nil {
		return fmt.Errorf("init repo: %w", err)
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
	if err := gr.AskForEncryption(c); err != nil {
		return err
	}

	if err := managementSelect(c, cfg); err != nil {
		return fmt.Errorf("select tracked: %w", err)
	}

	return nil
}

// gitCmd represents the git command.
func gitCommandFunc(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	if !files.Exists(cfg.Git.Path) {
		return git.ErrGitNotInitialized
	}

	gm, err := git.NewManager(cmd.Context(), cfg.Git.Path)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(args) == 0 {
		args = append(args, "log", "--oneline", "--reverse")
	}

	return gm.Exec(args...)
}

func pushFunc(ctx context.Context, cfg *config.Config) error {
	gr, err := git.NewRepo(cfg.DBPath)
	if err != nil {
		return err
	}
	remote, err := gr.Git.Remote()
	if err != nil || remote == "" {
		return git.ErrGitNoUpstream
	}

	// SetUpstream will push changes if upstream doesn't exist
	if err := git.SetUpstream(ctx, cfg.Git.Path); err != nil {
		if !errors.Is(err, git.ErrGitUpstreamExists) {
			return err
		}
	}

	// Check if there are unpushed commits
	proceed, err := gr.Git.HasUnpushedCommits()
	if err != nil {
		return err
	}
	if !proceed {
		return git.ErrGitUpToDate
	}

	// Update summary and push
	if err := git.UpdateSummaryAndCommit(gr, cfg.Info.Version); err != nil {
		return err
	}
	if err := gr.Git.Push(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func cloneFunc(cmd *cobra.Command, args []string) error {
	// FIX: prompt the user to select which repo to import or all.
	if len(args) == 0 {
		return git.ErrGitRepoURLEmpty
	}

	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
	c.Warning(fmt.Sprintf("This will clone into %q\n", cfg.Git.Path)).
		Warning("Recreate the databases and import all bookmarks\n").
		Warning("Set as remote origin\n").
		Flush()

	if !c.Confirm("continue?", "n") {
		return nil
	}

	if cfg.Flags.Force {
		_ = files.RemoveAll(cfg.Git.Path)
	}

	if files.Exists(cfg.Git.Path) {
		return fmt.Errorf("%w: %q", files.ErrPathExists, cfg.Git.Path)
	}

	cfg.Git.Remote = args[0]

	g, err := git.NewManager(cmd.Context(), cfg.Git.Path)
	if err != nil {
		return err
	}

	if err := g.Clone(cfg.Git.Remote); err != nil {
		return fmt.Errorf("cloning remote repo: %w", err)
	}

	rp := git.NewRepoProcessor(c, g, cfg, git.WithRPContext(cmd.Context()))

	return rp.Pull()
}
