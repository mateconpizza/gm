package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/files"
)

// commitCmd records staged changes in the repository.
var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "commit changes to the repository",
	RunE:  cli.HookGitSync,
}

// NewCmd is the git command.
func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:                "git",
		Short:              "git sync",
		Aliases:            []string{"g"},
		PersistentPreRunE:  cli.HookEnsureGitEnv,
		RunE:               gitCommandFunc,
		DisableFlagParsing: true,
	}

	cmds := []*cobra.Command{
		newInitRepoCmd(cfg),
		newTrackerCmd(cfg),
		newImportCmd(cfg),
		newCloneCmd(cfg),
		commitCmd,
		newPushCmd(cfg),
		newRawCmd(cfg),
	}

	for i := range cmds {
		cmdutil.HideFlag(cmds[i], "help")
	}
	c.AddCommand(cmds...)
	cmdutil.HideFlag(c, "help")

	return c
}

// newImportCmd clones a Git repository and imports bookmarks.
func newImportCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "import",
		Short: "import bookmarks from git",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Flags.Path == "" {
				return git.ErrGitRepoURLEmpty
			}

			a, cleanup, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cleanup()

			return importFromClone(a, cmd.Short)
		},
	}

	c.Flags().StringVarP(&cfg.Flags.Path, "uri", "i", "", "repo URI to import")

	return c
}

func newPushCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:                "push",
		Short:              "push changes to the repository",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return pushFunc(cmd.Context(), cfg)
		},
	}

	return c
}

// newInitRepoCmd initializes a new, empty Git repository.
func newInitRepoCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "init",
		Short: "create empty Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			gr, err := git.NewRepo(cfg.DBPath)
			if err != nil {
				return err
			}

			if err := gr.Git.Init(cfg.Flags.Reinit); err != nil {
				return fmt.Errorf("%w, use %s", err, ansi.BrightYellow.With(ansi.Italic).Sprint("--reinit"))
			}

			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			if err := gr.AskForEncryption(c); err != nil {
				return err
			}

			return managementSelect(c, cfg)
		},
	}

	c.Flags().BoolVar(&cfg.Flags.Reinit, "reinit", false, "reinitialize existing repository")

	return c
}

// newRawCmd proxies raw Git commands directly to the underlying git binary.
func newRawCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:                "raw",
		Short:              "raw git commands",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}

	return c
}

func newCloneCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "clone",
		Short: "clone a remote repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			// FIX: prompt the user to select which repo to import or all.
			if len(args) == 0 {
				return git.ErrGitRepoURLEmpty
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
		},
	}

	return c
}

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

// pushFunc pushes local changes to the remote repository.
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
