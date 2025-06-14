package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

type dbTrackerType struct {
	list    bool
	track   bool
	untrack bool
	mgt     bool
}

type gitFlagsType struct {
	origin string
	redo   bool
}

var gitFlags = gitFlagsType{}

var gitPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update remote refs along with associated objects",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.App
		repoPath := cfg.Path.Git
		procced, err := git.HasUnpushedCommits(repoPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if !procced {
			return git.ErrGitNoCommits
		}

		if err := handler.GitSummaryGenerate(cfg.Path.Data, repoPath, cfg.Info.Version); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := git.AddAll(repoPath); err != nil {
			return fmt.Errorf("staging summary: %w", err)
		}
		if err := git.CommitChanges(repoPath, "Updating Summary"); err != nil {
			return fmt.Errorf("committing summary: %w", err)
		}

		return git.PushChanges(repoPath)
	},
}

var gitCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return handler.GitCommit(config.App.DBPath, config.App.Path.Git, "Update")
	},
}

var gitRemoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage set of tracked repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := config.App.Path.Git
		if err := git.AddRemote(repoPath, gitFlags.origin); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := handler.GitSummaryGenerate(config.App.Path.Data, repoPath, config.App.Info.Version); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := git.AddAll(repoPath); err != nil {
			return fmt.Errorf("staging summary: %w", err)
		}
		if err := git.CommitChanges(repoPath, "Updating Summary"); err != nil {
			return fmt.Errorf("committing summary: %w", err)
		}
		if err := git.SetUpstream(repoPath); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

var (
	gitTrackerFlags = dbTrackerType{}

	gitTrackerCmd = &cobra.Command{
		Use:     "tracker",
		Short:   "Track database in git",
		Aliases: []string{"t"},
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := config.App.Path.Git
			tracked, err := git.Tracked(repoPath)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			f := frame.New(frame.WithColorBorder(color.Gray))
			t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))

			switch {
			case gitTrackerFlags.list:
				return ui.PrintGitRepoTracked(f, tracked)
			case gitTrackerFlags.mgt:
				return handler.GitTrackManagement(t, f, repoPath)
			case gitTrackerFlags.untrack:
				return handler.GitTrackRemoveRepo(config.App.DBName, repoPath, tracked)
			case gitTrackerFlags.track:
				return handler.GitTrackAddRepo(config.App.DBName, repoPath, tracked)
			}

			return cmd.Help()
		},
	}
)

var gitInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create empty Git repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))
		f := frame.New(frame.WithColorBorder(color.BrightBlue))

		repoPath := config.App.Path.Git

		if err := git.InitRepo(repoPath, gitFlags.redo); err != nil {
			return fmt.Errorf("init repo: %w", err)
		}

		tracked, err := handler.SelectecTrackedDB(t, f, repoPath)
		if err != nil {
			return fmt.Errorf("select tracked: %w", err)
		}

		if len(tracked) == 0 {
			return terminal.ErrActionAborted
		}

		if err := git.SetTracked(repoPath, tracked); err != nil {
			return fmt.Errorf("%w", err)
		}

		if t.Confirm(f.Question("Use GPG for encryption?").String(), "y") {
			if err := gpg.Init(repoPath); err != nil {
				return fmt.Errorf("gpg init: %w", err)
			}
		}

		return handler.GitInitTracking(repoPath, tracked)
	},
}

// gitCmd represents the git command.
var gitCmd = &cobra.Command{
	Use:                "git",
	Short:              "git commands",
	Aliases:            []string{"g"},
	DisableFlagParsing: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := handler.AssertDefaultDatabaseExists(); err != nil {
			return fmt.Errorf("%w", err)
		}
		switch cmd.Name() {
		case "init", "import":
			return nil
		}
		if !git.IsInitialized(config.App.Path.Git) {
			return git.ErrGitNotInitialized
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if slices.ContainsFunc([]string{"-h", "--help", "help"}, func(x string) bool {
			return slices.Contains(args, x)
		}) {
			return cmd.Help()
		}

		if len(args) == 0 {
			args = append(args, "log", "--oneline")
		}
		return git.RunGitCmd(config.App.Path.Git, args...)
	},
}

var gitImportCmd = &cobra.Command{
	Use:   "import",
	Short: "import bookmarks from git",
	RunE: func(cmd *cobra.Command, args []string) error {
		// FIX:
		// * Clone this repo into `config.App.Path.Git`
		// 	- Discard if already exists? Merge? Ignore?
		// * Maybe move this into subcommand `import`? added as a new source/way to
		// import?
		if len(args) == 0 {
			return git.ErrGitRepoURLEmpty
		}
		repoPathToClone := args[0]

		tmpPath := filepath.Join(os.TempDir(), config.App.Name+"-clone")
		if files.Exists(tmpPath) {
			if err := files.RemoveAll(tmpPath); err != nil {
				return fmt.Errorf("removing temp repo: %w", err)
			}
		}

		f := frame.New(frame.WithColorBorder(color.Gray))
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))

		if err := port.GitImport(t, f, tmpPath, repoPathToClone); err != nil {
			return fmt.Errorf("%w", err)
		}
		dbPath := config.App.DBPath
		if err := port.GitExport(dbPath); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := handler.GitCommit(dbPath, config.App.Path.Git, "Import from git"); err != nil {
			if errors.Is(err, git.ErrGitNothingToCommit) {
				return nil
			}

			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	gitRemoteCmd.Flags().StringVar(&gitFlags.origin, "add", "", "git remote origin")
	_ = gitRemoteCmd.MarkFlagRequired("add")

	gitInitCmd.Flags().BoolVar(&gitFlags.redo, "redo", false, "reinitialize")

	// tracker
	gitTrackerCmd.Flags().BoolVarP(&gitTrackerFlags.track, "track", "t", false, "track database in git")
	gitTrackerCmd.Flags().BoolVarP(&gitTrackerFlags.untrack, "untrack", "u", false, "untrack database in git")
	gitTrackerCmd.Flags().BoolVarP(&gitTrackerFlags.list, "list", "l", false, "list tracked databases")
	gitTrackerCmd.Flags().BoolVarP(&gitTrackerFlags.mgt, "manage", "m", false, "repos management in git")

	gitCmd.AddCommand(gitPushCmd, gitCommitCmd, gitInitCmd, gitImportCmd, gitTrackerCmd)
	rootCmd.AddCommand(gitCmd)
}
