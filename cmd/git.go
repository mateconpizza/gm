package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

type gitFlagsType struct {
	origin string
}

var gitFlags = gitFlagsType{}

var gitPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch from and integrate with another repository or a local branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		return git.Fetch(config.App.Path.Git)
	},
}

var gitPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update remote refs along with associated objects",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := config.App.Path.Git
		procced, err := git.HasUnpushedCommits(repoPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if !procced {
			return git.ErrGitNoCommits
		}

		if err := handler.GitSummaryGenerate(repoPath); err != nil {
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
		return handler.GitCommit("Update")
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
		if err := handler.GitSummaryGenerate(repoPath); err != nil {
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

var gitLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show commit logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return git.Log(config.App.Path.Git)
	},
}

var gitInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create empty Git repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))
		f := frame.New(frame.WithColorBorder(color.BrightBlue))

		repoPath := config.App.Path.Git

		if err := git.InitRepo(repoPath, config.App.Force); err != nil {
			return fmt.Errorf("init repo: %w", err)
		}

		tracked, err := handler.SelectecTrackedDB(t, f, repoPath)
		if err != nil {
			return fmt.Errorf("select tracked: %w", err)
		}

		return initializeTracking(t, f, tracked)
	},
}

// initializeTracking will initialize the tracking database.
func initializeTracking(t *terminal.Term, f *frame.Frame, tracked []string) error {
	s := f.Clear().Question("Use GPG for encryption?").String()

	if gpg.IsInitialized(config.App.Path.Git) || !t.Confirm(s, "y") {
		for _, dbFile := range tracked {
			if err := port.GitExport(dbFile); err != nil {
				if errors.Is(err, git.ErrGitNothingToCommit) {
					f.Clear().
						Warning(fmt.Sprintf("Skipping %q, no bookmarks found\n", filepath.Base(dbFile))).
						Flush()
					continue
				}
				return fmt.Errorf("%w", err)
			}
			if err := handler.GitCommit("Initializing repo"); err != nil {
				return fmt.Errorf("%w", err)
			}
		}
		success := color.BrightGreen("Successfully").Italic().String()
		f.Clear().Success(success + color.Text(" initialized\n").Italic().String()).Flush()

		return nil
	}

	for _, dbFile := range tracked {
		config.SetDBName(filepath.Base(dbFile))
		config.SetDBPath(dbFile)
		if err := gpgInitCmd.RunE(&cobra.Command{}, []string{}); err != nil {
			return fmt.Errorf("gpg init: %w", err)
		}
	}

	return nil
}

// gitCmd represents the git command.
var gitCmd = &cobra.Command{
	Use:     "git",
	Short:   "Git commands",
	Aliases: []string{"g"},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := handler.AssertDefaultDatabaseExists(); err != nil {
			return fmt.Errorf("%w", err)
		}
		switch cmd.Name() {
		case "init", "git", "clone":
			return nil
		}
		if !git.IsInitialized(config.App.Path.Git) {
			return git.ErrGitNotInitialized
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

var gpgInitCmd = &cobra.Command{
	Use:    "gpg",
	Short:  "Initialize a git GPG repository",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := config.App.Path.Git
		if err := gpg.Init(repoPath); err != nil {
			return fmt.Errorf("gpg init: %w", err)
		}

		success := color.BrightGreen("Successfully").Italic().String()
		f := frame.New(frame.WithColorBorder(color.Gray))
		f.Clear().Success(success + color.Text(" initialized\n").Italic().String()).Flush()

		if err := port.GitExport(config.App.DBPath); err != nil {
			if errors.Is(err, git.ErrGitNothingToCommit) {
				f.Clear().
					Warning(fmt.Sprintf("Skipping %q, no bookmarks found\n", filepath.Base(config.App.DBPath))).
					Flush()
				return nil
			}

			return fmt.Errorf("%w", err)
		}

		if err := handler.GitCommit("Initializing encrypted repo"); err != nil {
			if errors.Is(err, git.ErrGitNothingToCommit) {
				return nil
			}
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

var gitCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a repository",
	Long:  "Clone a repository and import to the a new bookmarks database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// FIX:
		// * Clone this repo into `config.App.Path.Git`
		// 	- Discard if already exists? Merge? Ignore?
		// * Maybe move this into subcommand `import`? added as a new source/way to
		// import?
		if len(args) == 0 {
			return git.ErrGitRepoURLEmpty
		}
		repoPath := args[0]

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

		if err := port.GitImport(t, f, tmpPath, repoPath); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := port.GitExport(config.App.DBPath); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := handler.GitCommit("Import from git"); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	gitRemoteCmd.Flags().StringVar(&gitFlags.origin, "add", "", "git remote origin")
	_ = gitRemoteCmd.MarkFlagRequired("add")

	gitCmd.AddCommand(gitPullCmd, gitPushCmd, gitCommitCmd, gitRemoteCmd,
		gitInitCmd, gitLogCmd, gpgInitCmd, gitCloneCmd)
	rootCmd.AddCommand(gitCmd)
}
