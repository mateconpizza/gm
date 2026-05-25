package git

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
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

			g, err := git.New(cmd.Context(), app.Git.Path)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			if len(args) == 0 {
				args = append(args, "log", "--oneline", "--reverse")
			}

			return g.Exec(args...)
		},
	}
	c.AddCommand(
		newInitRepoCmd(app),
		newTrackerCmd(app),
		newCloneCmd(app),
		commitCmd,
		newPushCmd(app),
		newRawCmd(app),
		newDisableCmd(app),
	)

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
			m, err := git.NewManager(app.Path.Database)
			if err != nil {
				return err
			}

			if err := m.Git.Init(app.Flags.Reinit); err != nil {
				return fmt.Errorf("%w, use %s", err, ansi.BrightYellow.With(ansi.Italic).Sprint("--reinit"))
			}

			c := ui.NewDefaultConsole(cmd.Context(), func(err error) { sys.ErrAndExit(err) })
			if err := m.AskForEncryption(c, app); err != nil {
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

			g, err := git.New(cmd.Context(), app.Git.Path)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			if len(args) == 0 {
				args = append(args, "log", "--oneline", "--reverse")
			}

			return g.Exec(args...)
		},
	}

	return c
}

func newCloneCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "clone",
		Short:   "import from remote",
		Aliases: []string{"import"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cleanup, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cleanup()

			app.Git.Remote = args[0]

			return handler.GitClone(d)
		},
	}

	return c
}

func newDisableCmd(_ *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "disable",
		Short: "disable tracking (wip)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented yet...")
			return nil
		},
	}

	return c
}

// pushFunc pushes local changes to the remote repository.
func pushFunc(ctx context.Context, app *application.App) error {
	m, err := git.NewManager(app.Path.Database)
	if err != nil {
		return err
	}
	remote, err := m.Git.Remote()
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
	proceed, err := m.Git.HasUnpushedCommits()
	if err != nil {
		return err
	}
	if !proceed {
		return git.ErrGitUpToDate
	}

	// Update summary and push
	if err := git.UpdateSummaryAndCommit(m, app.Info.Version); err != nil {
		return err
	}
	if err := m.Git.Push(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}
