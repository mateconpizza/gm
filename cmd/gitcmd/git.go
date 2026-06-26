package gitcmd

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

// NewCmd is the git command.
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "git",
		Short:              "git operations",
		Aliases:            []string{"g"},
		DisableFlagParsing: true,
		PersistentPreRunE:  cli.HookGitEnsureEnv(app),
		PreRun:             cli.HookGitEnableLogging(app),
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
		newInitRepoCmd(app), // initialize a bookmarks repository
		newCloneCmd(app),    // clone bookmarks from a remote repository
		newEnableCmd(app),   // enable git integration
		newDisableCmd(app),  // disable git integration
		newTrackerCmd(app),  // configure repository tracking
		newLoggingCmd(app),  // configure git command logging
		newCommitCmd(app),   // commit bookmark database changes
		newPushCmd(app),     // push bookmark changes to a remote
		newSyncCmd(app),     // synchronize bookmarks with the repository
		newInfoCmd(app),     // show repository status and configuration
		newRawCmd(app),      // run arbitrary git commands
	)

	return c
}

// commitCmd records staged changes in the repository.
func newCommitCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:    "commit",
		Short:  "commit bookmark database changes",
		PreRun: cli.HookGitEnableLogging(app),
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

func newPushCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:                "push",
		Short:              "push bookmark changes to a remote",
		DisableFlagParsing: true,
		PreRun:             cli.HookGitEnableLogging(app),
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
		Short:       "initialize a bookmarks repository",
		Annotations: cli.SkipGitSync,
		PreRun:      cli.HookGitEnableLogging(app),
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
		Short:              "run arbitrary git commands",
		DisableFlagParsing: true,
		PreRun:             cli.HookGitEnableLogging(app),
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
		Short:              "clone bookmarks from a remote repository",
		Aliases:            []string{"import"},
		Args:               cobra.MinimumNArgs(1),
		PersistentPostRunE: cli.HookGitSync(app),
		PreRun:             cli.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cleanup, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cleanup()

			app.Git.Remote = args[0]

			return gitops.Clone(cmd.Context(), d)
		},
	}

	return c
}

func newDisableCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "disable",
		Short:       "disable git integration",
		Annotations: cli.SkipGitCheck,
		Aliases:     []string{"off"},
		PostRun:     cli.HookGitStatus(app),
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
		Short:              "enable git integration",
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
		Use:    "sync",
		Short:  "synchronize bookmarks with the repository",
		PreRun: cli.HookGitEnableLogging(app),
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

func newLoggingCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:               "logging",
		Short:             "configure git command logging",
		PersistentPostRun: cli.HookGitLoggingStatus(app),
		Example: app.Example(`  $ {cmd} git logging enable
  $ {cmd} git logging on
  $ {cmd} git logging disable
  $ {cmd} git logging off`),
	}

	cmdutil.HideFlag(c, "db", "color", "yes", "force")

	c.AddCommand(&cobra.Command{
		Use:     "enable",
		Short:   "enable logging",
		Aliases: []string{"on"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Git.Log {
				slog.Warn("git: output logging already enable")
				return sys.ErrExitFailure
			}

			app.Git.Log = true
			slog.Warn("git: output logging enabled")
			return app.WriteConfig(true)
		},
	})

	c.AddCommand(&cobra.Command{
		Use:     "disable",
		Short:   "disable logging",
		Aliases: []string{"off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Git.Log {
				slog.Warn("git: output logging already disable")
				return sys.ErrExitFailure
			}

			app.Git.Log = false
			slog.Warn("git: output logging disabled")
			return app.WriteConfig(true)
		},
	})

	return c
}

func newInfoCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "info",
		Short:   "show repository status and configuration",
		Example: app.Example(`  $ {cmd} git info`),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cleanup, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cleanup()

			return gitops.InfoCmd(cmd.Context(), d)
		},
	}

	cmdutil.HideFlag(c, "db", "color", "yes", "force")

	return c
}
