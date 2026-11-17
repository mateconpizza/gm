package gitcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/command"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/ansi"
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
		PersistentPreRunE:  command.HookGitEnsureEnv(app),
		PreRun:             command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := git.New(
				app.Path.Git(),
				gitops.NewGitCommandLogger(app.Flags.Color),
				git.WithLogger(app.Logger()),
				git.WithWriter(app.Git.Writer()),
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

	command.HideFlag(c, "reverse", "filter")

	return c
}

// commitCmd records staged changes in the repository.
func newCommitCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:    "commit",
		Short:  "commit changes",
		PreRun: command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.Execute(cmd.Context(), args, nil, gitops.CommitChanges)
		},
	}
}

func newPushCmd(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:                "push",
		Short:              "push bookmark changes to a remote",
		DisableFlagParsing: true,
		PreRun:             command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			gm, err := app.Git.Manager()
			if err != nil {
				return err
			}
			return gm.Push(cmd.Context())
		},
	}
}

// newInitRepoCmd initializes a new, empty Git repository.
func newInitRepoCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "init",
		Short:       "initialize a bookmarks repository",
		Annotations: command.SkipGitSync,
		PreRun:      command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.Run(cmd.Context(), args, func(ctx context.Context, d *deps.Deps) error {
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
		PreRun:             command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := git.New(
				app.Path.Git(),
				gitops.NewGitCommandLogger(app.Flags.Color),
				git.WithLogger(app.Logger()),
				git.WithWriter(app.Git.Writer()),
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
	return &cobra.Command{
		Use:                "clone",
		Short:              "import git",
		Long:               "clone bookmarks from a remote repository",
		Aliases:            []string{"import"},
		Args:               cobra.MinimumNArgs(1),
		PersistentPostRunE: command.HookGitSync(app),
		PreRun:             command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			app.Git.Remote = args[0]
			return command.Run(cmd.Context(), args, gitops.Clone)
		},
	}
}

func newDisableCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "disable",
		Short:       "disable git integration",
		Annotations: command.SkipGitCheck,
		Aliases:     []string{"off"},
		PostRun:     command.HookGitStatus(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := app.Logger().With("command", cmd.CommandPath())
			if !app.GitEnabled() {
				log.Error("git: already disable")
				return app.Failure()
			}

			app.Git.Enabled = false
			return app.WriteConfig(true)
		},
	}

	command.HideFlag(c, "db", "color", "yes", "force")

	return c
}

func newEnableCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "enable",
		Short:       "enable git integration",
		Annotations: command.SkipGitCheck,
		Aliases:     []string{"on"},
		PersistentPostRunE: command.ChainHooks(
			command.HookGitManager(app),
			command.HookGitPrune(app),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			log := app.Logger().With("command", cmd.CommandPath())
			if app.GitEnabled() {
				log.Warn("git: already enabled")
				return app.Failure()
			}

			app.Git.Enabled = true
			return app.WriteConfig(true)
		},
	}

	command.HideFlag(c, "db", "color", "yes", "force")

	return c
}

func newSyncCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:    "sync",
		Short:  "synchronize bookmarks with the repository",
		PreRun: command.HookGitEnableLogging(app),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			log := app.Logger().With("cmd", "sync", "all", app.Flags.All)
			log.Debug("starting sync")

			if app.Flags.All {
				return gitops.SyncAll(ctx, app.Git.Writer(), app, ui.NewDefaultConsole(app, app.Exit))
			}

			r, err := db.Open(ctx, app.Path.DB())
			if err != nil {
				return err
			}
			defer r.Close()

			bs, err := r.All(ctx)
			if err != nil {
				return err
			}
			log = log.With("db", r.BaseName())
			log.Debug("loaded bookmarks", "count", len(bs))

			sess := app.Session().
				WithConfirmation(false).
				WithValidation(true).
				WithLogger(log)

			gm, err := app.Git.Manager()
			if err != nil {
				return err
			}

			gr := gitops.NewRepo(gm, r.BaseName(), sess, app.Flags.Color)
			gitops.RegisterHooks(r, app, gm, gr)

			if err := gitops.PruneRepo(ctx, gm, gr, bs, log); err != nil {
				return err
			}

			log.Debug("sync finished")

			return nil
		},
	}

	c.Flags().BoolVar(&app.Flags.All, "all", false, "sync all tracked databases")

	return c
}

func newLoggingCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:               "logging",
		Short:             "configure git command logging",
		PersistentPostRun: command.HookGitLoggingStatus(app),
		Example: app.Example(`  $ {cmd} git logging enable
  $ {cmd} git logging on
  $ {cmd} git logging disable
  $ {cmd} git logging off`),
	}

	command.HideFlag(c, "db", "color", "yes", "force")

	c.AddCommand(&cobra.Command{
		Use:     "enable",
		Short:   "enable logging",
		Aliases: []string{"on"},
		RunE: func(cmd *cobra.Command, args []string) error {
			log := app.Logger().With("command", cmd.CommandPath())
			if app.Git.Log {
				log.Warn("git: output logging already enable")
				return app.Failure()
			}

			app.Git.Log = true
			log.Warn("git: output logging enabled")
			return app.WriteConfig(true)
		},
	})

	c.AddCommand(&cobra.Command{
		Use:     "disable",
		Short:   "disable logging",
		Aliases: []string{"off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			log := app.Logger().With("command", cmd.CommandPath())
			if !app.Git.Log {
				log.Warn("git: output logging already disable")
				return app.Failure()
			}

			app.Git.Log = false
			log.Warn("git: output logging disabled")
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
			return command.Run(cmd.Context(), args, gitops.InfoCmd)
		},
	}

	command.HideFlag(c, "db", "yes", "force")

	return c
}

func newSetCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "set",
		Short:       "configuration",
		Aliases:     []string{"cfg", "config"},
		Annotations: command.SkipFormatter,
		Example: app.Example(`  $ {cmd} git set logging enable
  $ {cmd} git set logging disable`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	c.AddCommand(
		newLoggingCmd(app), // configure git command logging
	)
	command.HideFlag(c, "color", "db", "menu", "head", "tail", "fields", "sort", "tag", "filter")

	return c
}

func newReadmeCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:               "readme",
		Short:             "repository README.md",
		PersistentPreRunE: command.HookGitReadmeEnable(app),
		Run: func(cmd *cobra.Command, args []string) {
			s := "enabled"
			if !app.Git.Readme.Enabled {
				s = "disabled"
			}
			fmt.Fprintln(os.Stdout, s)
		},
	}

	command.HideFlag(c, "filter", "reverse")

	app.Session().
		WithValidation(true)

	c.AddCommand(
		newReadmeEnable(app),
		newReadmeDisable(app),
		newReadmeView(app),
		newReadmeConfigure(app),
		newReadmeSync(app),
	)
	return c
}

func newReadmeEnable(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:         "enable",
		Short:       "enable repository README.md",
		Aliases:     []string{"on"},
		Annotations: command.SkipGitReadmeCheck,
		RunE: func(cmd *cobra.Command, args []string) error {
			log := app.Logger().With("command", cmd.CommandPath())
			if app.Git.Readme.Enabled {
				log.Warn("git: readme already enable")
				return app.Failure()
			}
			return command.Run(cmd.Context(), args, func(ctx context.Context, d *deps.Deps) error {
				return gitops.EnableReadme(ctx, app, d.Console())
			})
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
			log := app.Logger().With("command", cmd.CommandPath())
			if !app.Git.Readme.Enabled {
				log.Warn("git: readme already disable")
				return app.Failure()
			}
			return gitops.DisableReadme(cmd.Context(), app, ui.NewDefaultConsole(app, app.Exit))
		},
	}
}

func newReadmeSync(app *application.App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "sync with database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.Run(cmd.Context(), args, func(ctx context.Context, d *deps.Deps) error {
				return gitops.EnableReadme(ctx, app, d.Console())
			})
		},
	}
}

func newReadmeView(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "view",
		Aliases: []string{"show"},
		Short:   "Display the repository README",
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitops.ViewReadme(cmd.Context(), app)
		},
	}
	command.HideFlag(c, "filter", "yes", "force", "reverse")
	return c
}

func newReadmeConfigure(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "configure",
		Aliases: []string{"config", "cfg"},
		Short:   "Configure README settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.Run(cmd.Context(), args, func(ctx context.Context, d *deps.Deps) error {
				return gitops.ConfigureReadme(ctx, app, d.Console())
			})
		},
	}
	command.HideFlag(c, "filter", "yes", "force", "reverse")
	return c
}
