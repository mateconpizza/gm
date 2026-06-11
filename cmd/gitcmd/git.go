package gitcmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

// commitCmd records staged changes in the repository.
func newCommitCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "commit",
		Short: "commit changes to the repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			r, err := db.New(cmd.Context(), app.Path.DB())
			if err != nil {
				return err
			}

			gr := gitops.NewRepo(m, r.Name(), git.WithRepoStore(r))
			return m.SaveChanges(cmd.Context(), gr, cmd.Short)
		},
	}
}

// NewCmd is the git command.
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "git",
		Short:              "git operations",
		Aliases:            []string{"g"},
		DisableFlagParsing: true,
		PersistentPreRunE:  cli.HookEnsureGitEnv(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := gitops.NewGit(app)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				args = append(args, "log", "--oneline", "--reverse")
			}

			return g.Exec(cmd.Context(), args...)
		},
	}
	c.AddCommand(
		newInitRepoCmd(app),
		newTrackerCmd(app),
		newCloneCmd(app),
		newCommitCmd(app),
		newPushCmd(app),
		newRawCmd(app),
		newDisableCmd(app),
		newEnableCmd(app),
		newSyncCmd(app),
	)

	return c
}

func newPushCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "push",
		Short:              "push changes to the repository",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			return gitops.Push(cmd.Context(), app, m)
		},
	}

	return c
}

// newInitRepoCmd initializes a new, empty Git repository.
func newInitRepoCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "init",
		Short:       "create empty Git repository",
		Annotations: cli.SkipGitSync,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			return gitops.Init(cmd.Context(), app, m)
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
			g, err := gitops.NewGit(app)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				args = append(args, "log", "--oneline", "--reverse")
			}

			return g.Exec(cmd.Context(), args...)
		},
	}

	return c
}

func newCloneCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "clone",
		Short:              "import from remote",
		Aliases:            []string{"import"},
		Args:               cobra.MinimumNArgs(1),
		PersistentPostRunE: cli.HookGitSync(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cleanup, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cleanup()

			app.Git.Remote = args[0]

			return handler.GitClone(cmd.Context(), d)
		},
	}

	return c
}

func newDisableCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "disable",
		Short:       "disable git tracking",
		Annotations: cli.SkipGitCheck,
		Aliases:     []string{"off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.GitEnabled() {
				slog.Warn("git: already disable")
				return sys.ErrExitFailure
			}

			app.Git.Enabled = false
			return app.WriteConfig(true)
		},
	}

	cmdutil.HideFlag(c, "db", "color", "yes", "force")

	return c
}

func newEnableCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "enable",
		Short:              "enable git tracking",
		Annotations:        cli.SkipGitCheck,
		PersistentPostRunE: cli.HookGitPrune(app),
		Aliases:            []string{"on"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.GitEnabled() {
				slog.Warn("git: already enabled")
				return sys.ErrExitFailure
			}

			app.Git.Enabled = true
			return app.WriteConfig(true)
		},
	}

	cmdutil.HideFlag(c, "db", "color", "yes", "force")

	return c
}

func newSyncCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "sync bookmarks with local repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := db.New(cmd.Context(), app.Path.DB())
			if err != nil {
				return err
			}
			defer r.Close()

			return gitops.Prune(cmd.Context(), app, r)
		},
	}

	return c
}
