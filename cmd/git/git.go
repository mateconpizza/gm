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
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
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
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "git",
		Short:              "git sync",
		Aliases:            []string{"g"},
		PersistentPreRunE:  cli.HookEnsureGitEnv,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !files.Exists(app.Git.Path) {
				return git.ErrGitNotInitialized
			}

			gm, err := git.NewManager(cmd.Context(), app.Git.Path)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			if len(args) == 0 {
				args = append(args, "log", "--oneline", "--reverse")
			}

			return gm.Exec(args...)
		},
	}

	cmds := []*cobra.Command{
		newInitRepoCmd(app),
		newTrackerCmd(app),
		newImportCmd(app),
		newCloneCmd(app),
		commitCmd,
		newPushCmd(app),
		newRawCmd(app),
	}

	for i := range cmds {
		cmdutil.HideFlag(cmds[i], "help")
	}
	c.AddCommand(cmds...)
	cmdutil.HideFlag(c, "help")

	return c
}

// newImportCmd clones a Git repository and imports bookmarks.
func newImportCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "import",
		Short: "import bookmarks from git",
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Flags.Path == "" {
				return git.ErrGitRepoURLEmpty
			}

			d, cleanup, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cleanup()

			return importFromClone(d, cmd.Short)
		},
	}

	c.Flags().StringVarP(&app.Flags.Path, "uri", "i", "", "repo URI to import")

	return c
}

func newPushCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "push",
		Short:              "push changes to the repository",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return pushFunc(cmd.Context(), app)
		},
	}

	return c
}

// newInitRepoCmd initializes a new, empty Git repository.
func newInitRepoCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "init",
		Short: "create empty Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			gr, err := git.NewRepo(app.Path.Database)
			if err != nil {
				return err
			}

			if err := gr.Git.Init(app.Flags.Reinit); err != nil {
				return fmt.Errorf("%w, use %s", err, ansi.BrightYellow.With(ansi.Italic).Sprint("--reinit"))
			}

			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			if err := gr.AskForEncryption(c); err != nil {
				return err
			}

			return managementSelect(c, app)
		},
	}

	c.Flags().BoolVar(&app.Flags.Reinit, "reinit", false, "reinitialize existing repository")

	return c
}

// newRawCmd proxies raw Git commands directly to the underlying git binary.
func newRawCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "raw",
		Short:              "raw git commands",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !files.Exists(app.Git.Path) {
				return git.ErrGitNotInitialized
			}

			gm, err := git.NewManager(cmd.Context(), app.Git.Path)
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

func newCloneCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "clone",
		Short: "clone a remote repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			// FIX: prompt the user to select which repo to import or all.
			if len(args) == 0 {
				return git.ErrGitRepoURLEmpty
			}

			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			c.Warning(fmt.Sprintf("This will clone into %q\n", app.Git.Path)).
				Warning("Recreate the databases and import all bookmarks\n").
				Warning("Set as remote origin\n").
				Flush()

			if !c.Confirm("continue?", "n") {
				return nil
			}

			if app.Flags.Force {
				_ = files.RemoveAll(app.Git.Path)
			}

			if files.Exists(app.Git.Path) {
				return fmt.Errorf("%w: %q", files.ErrPathExists, app.Git.Path)
			}

			app.Git.Remote = args[0]

			g, err := git.NewManager(cmd.Context(), app.Git.Path)
			if err != nil {
				return err
			}

			if err := g.Clone(app.Git.Remote); err != nil {
				return fmt.Errorf("cloning remote repo: %w", err)
			}

			rp := git.NewRepoProcessor(c, g, app, git.WithRPContext(cmd.Context()))

			return rp.Pull()
		},
	}

	return c
}

// importFromClone clones a git repo and imports its bookmarks.
func importFromClone(d *deps.Deps, commitMesg string) error {
	app := d.App
	tmpPath := filepath.Join(os.TempDir(), app.Name+"-clone")
	if files.Exists(tmpPath) {
		_ = files.RemoveAll(tmpPath)
	}
	defer func() { _ = files.RemoveAll(tmpPath) }()

	d.SetConsole(ui.NewDefaultConsole(d.Context(), func(err error) {
		fmt.Println("cleaning up temp files...")
		if err := files.RemoveAll(tmpPath); err != nil {
			slog.Error("cleaning up temp dir", "path", tmpPath)
		}
		sys.ErrAndExit(err)
	}))

	gm, err := git.NewManager(d.Context(), tmpPath)
	if err != nil {
		return err
	}

	imported, err := git.Import(d, gm)
	if err != nil {
		return err
	}
	if !app.Git.Enabled {
		slog.Warn("git import: repo not initialized", "path", app.Git.Path)
		return nil
	}

	return processImported(d.Console(), imported, commitMesg)
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

// pushFunc pushes local changes to the remote repository.
func pushFunc(ctx context.Context, app *application.App) error {
	gr, err := git.NewRepo(app.Path.Database)
	if err != nil {
		return err
	}
	remote, err := gr.Git.Remote()
	if err != nil || remote == "" {
		return git.ErrGitNoUpstream
	}

	// SetUpstream will push changes if upstream doesn't exist
	if err := git.SetUpstream(ctx, app.Git.Path); err != nil {
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
	if err := git.UpdateSummaryAndCommit(gr, app.Info.Version); err != nil {
		return err
	}
	if err := gr.Git.Push(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}
