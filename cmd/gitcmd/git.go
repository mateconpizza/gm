package gitcmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
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
			g, err := gitops.NewGit(
				app.Git.Writer(),
				app.Path.Git(),
				gitops.NewGitCommandLogger(app.Flags.Color),
			)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return gitops.StreamLog(cmd.Context(), g, ansi.NewPalette(app.Flags.Color))
			}

			return g.Exec(cmd.Context(), args...)
		},
	}

	c.AddCommand(
		newInitRepoCmd(app), // initialize a bookmarks repository
		newTrackerCmd(app),  // configure repository tracking
		newEnableCmd(app),   // enable git integration
		newDisableCmd(app),  // disable git integration
		newInfoCmd(app),     // show repository status and configuration
		newReadmeCmd(app),   // generate a README.md
		newCloneCmd(app),    // clone bookmarks from a remote repository
		newCommitCmd(app),   // commit bookmark database changes
		newPushCmd(app),     // push bookmark changes to a remote
		newSyncCmd(app),     // synchronize bookmarks with the repository
		newRawCmd(app),      // run arbitrary git commands
		newSetCmd(app),      // configuration
	)

	cmdutil.HideFlag(c, "reverse", "filter")

	return c
}

// commitCmd records staged changes in the repository.
func newCommitCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:    "commit",
		Short:  "commit changes",
		PreRun: cli.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Execute(cmd.Context(), args, nil, gitops.CommitChanges)
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
			return app.Git.Manager().Push(cmd.Context())
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
			return cmdutil.Run(cmd.Context(), args, func(ctx context.Context, d *deps.Deps) error {
				gm, err := gitops.NewManager(app)
				if err != nil {
					return err
				}

				app.Git.SetManager(gm)

				c := d.Console()
				if err := gitops.Init(cmd.Context(), app, c); err != nil {
					return err
				}

				fmt.Fprintln(os.Stdout)

				return gitops.TrackMgr(cmd.Context(), app, gm, c, app.Databases())
			})
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
			g, err := gitops.NewGit(
				app.Git.Writer(),
				app.Path.Git(),
				gitops.NewGitCommandLogger(app.Flags.Color),
			)
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
		Short:              "import git",
		Long:               "clone bookmarks from a remote repository",
		Aliases:            []string{"import"},
		Args:               cobra.MinimumNArgs(1),
		PersistentPostRunE: cli.HookGitSync(app),
		PreRun:             cli.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			app.Git.Remote = args[0]
			return cmdutil.Run(cmd.Context(), args, gitops.Clone)
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
				return app.Failure()
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
		Use:         "enable",
		Short:       "enable git integration",
		Annotations: cli.SkipGitCheck,
		Aliases:     []string{"on"},
		PersistentPostRunE: cli.ChainHooks(
			cli.HookGitManager(app),
			cli.HookGitPrune(app),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.GitEnabled() {
				slog.Warn("git: already enabled")
				return app.Failure()
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
			return cmdutil.Execute(cmd.Context(), args, nil, func(ctx context.Context, d *deps.Deps, bs []*bookmark.Bookmark) error {
				if app.Flags.All {
					return cmdutil.Run(ctx, args, gitops.SyncAll)
				}

				r, err := d.Repository()
				if err != nil {
					return err
				}
				defer r.Close()

				sess := app.Session().
					WithConfirmation(false).
					WithValidation(true)

				gm := app.Git.Manager()
				gr := gitops.NewRepo(gm, r.BaseName(), sess, app.Flags.Color)
				gitops.RegisterHooks(r, app, gr)

				return gitops.PruneRepo(ctx, gm, gr, bs)
			})
		},
	}

	c.Flags().BoolVar(&app.Flags.All, "all", false, "sync all tracked databases")

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
				return app.Failure()
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
				return app.Failure()
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
		Aliases: []string{"view", "stats"},
		Example: app.Example(`  $ {cmd} git info`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.Run(cmd.Context(), args, gitops.InfoCmd)
		},
	}

	cmdutil.HideFlag(c, "db", "yes", "force")

	return c
}

func newSetCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "set",
		Short:       "configuration",
		Aliases:     []string{"cfg", "config"},
		Annotations: cli.SkipFormatter,
		Example: app.Example(`  $ {cmd} git set logging enable
  $ {cmd} git set logging disable`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	c.AddCommand(
		newLoggingCmd(app), // configure git command logging
	)
	cmdutil.HideFlag(c, "color", "db", "menu", "head", "tail", "fields", "sort", "tag", "filter")

	return c
}

func newReadmeCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "readme",
		Short: "repository README.md",
		Run: func(cmd *cobra.Command, args []string) {
			newReadmeStatus(app).Run(cmd, args)
		},
	}

	cmdutil.HideFlag(c, "filter", "reverse")
	c.Flags().BoolVarP(&app.Flags.Configure, "reconfigure", "r", false, "reconfigure opts")

	app.Session().
		WithValidation(true)

	c.AddCommand(
		newReadmeEnable(app),
		newReadmeDisable(app),
		newReadmeStatus(app),
		newReadmeSync(app),
	)
	return c
}

func newReadmeEnable(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "enable",
		Short:   "enable repository README.md",
		Aliases: []string{"on"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Git.Readme.Enabled {
				slog.Warn("git: readme already enable")
				return app.Failure()
			}
			return cmdutil.Run(cmd.Context(), args, gitops.EnableReadme)
		},
	}
	return c
}

func newReadmeDisable(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:     "disable",
		Short:   "disable repository README.md",
		Aliases: []string{"off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Git.Readme.Enabled {
				slog.Warn("git: readme already disable")
				return app.Failure()
			}
			return cmdutil.Run(cmd.Context(), args, gitops.DisableReadme)
		},
	}
}

func newReadmeSync(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "sync with database",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Git.Readme.Enabled {
				slog.Warn("git: readme disabled")
				return app.Failure()
			}
			return cmdutil.Run(cmd.Context(), args, gitops.EnableReadme)
		},
	}
}

func newReadmeStatus(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "show status",
		Run: func(cmd *cobra.Command, args []string) {
			s := "enabled"
			if !app.Git.Readme.Enabled {
				s = "disabled"
			}
			fmt.Fprintln(os.Stdout, s)
		},
	}
	return c
}
