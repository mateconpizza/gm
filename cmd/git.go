package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/importer"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

type gitFlagsType struct {
	origin string
	repo   string
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
		dbName := files.StripSuffixes(config.App.DBName)
		summaryPath := filepath.Join(repoPath, dbName, "summary.json")

		// Generate the new summary
		newSum, err := handler.GitSummary(repoPath)
		if err != nil {
			return fmt.Errorf("generating summary: %w", err)
		}

		// Determine whether to write summary
		writeAndCommit := func() error {
			if err := files.JSONWrite(summaryPath, newSum, true); err != nil {
				return fmt.Errorf("writing summary: %w", err)
			}
			if err := git.AddAll(repoPath); err != nil {
				return fmt.Errorf("staging summary: %w", err)
			}
			msg := fmt.Sprintf("[%s]: Updating Summary", config.App.DBName)
			if err := git.CommitChanges(repoPath, msg); err != nil {
				return fmt.Errorf("committing summary: %w", err)
			}
			return nil
		}

		// Check if summary file exists
		if !files.Exists(summaryPath) {
			if err := writeAndCommit(); err != nil {
				return err
			}
		} else {
			oldSum := git.NewSummary()
			if err := files.JSONRead(summaryPath, oldSum); err != nil {
				return fmt.Errorf("reading existing summary: %w", err)
			}
			if newSum.Checksum != oldSum.Checksum {
				if err := writeAndCommit(); err != nil {
					return err
				}
			}
		}

		// Push all changes
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
		return git.AddRemote(config.App.Path.Git, gitFlags.origin)
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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if files.Exists(config.App.Path.Git) {
			return nil
		}
		return files.MkdirAll(config.App.Path.Git)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))

		repoPath := config.App.Path.Git
		init, err := git.InitRepo(repoPath, config.App.Force)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		f := frame.New(frame.WithColorBorder(color.Gray))
		if init != "" {
			f.Midln(init)
		}

		if !t.Confirm(f.Question("Use GPG to encrypt the repository?").String(), "y") {
			if err := handler.GitCommit("Initializing repo"); err != nil {
				return fmt.Errorf("%w", err)
			}
			success := color.BrightGreen("Successfully").Italic().String()
			f.Clear().Success(success + color.Text(" initialized\n").Italic().String()).Flush()

			return nil
		}
		return gpgInitCmd.RunE(cmd, args)
	},
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
		if err := gpg.Init(config.App.Path.Git); err != nil {
			return fmt.Errorf("gpg init: %w", err)
		}
		success := color.BrightGreen("Successfully").Italic().String()
		f := frame.New(frame.WithColorBorder(color.Gray))
		f.Clear().Success(success + color.Text(" initialized\n").Italic().String()).Flush()
		return handler.GitCommit("Initializing encrypted repo")
	},
}

var gitCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a repository",
	Long:  "Clone a repository and import to the a new bookmarks database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// FIX:
		// * Clone this repo into `config.App.Path.Git`? Discard if already exists?
		// 	- Discard if already exists? Merge? Ignore?
		// * Maybe move this into subcommand `import`? added as a new source/way to
		// import?
		if len(args) == 0 {
			return ErrImportGitRepoNotFound
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

		if err := importer.Git(tmpPath, repoPath, f, t); err != nil {
			return fmt.Errorf("importing from repo: %w", err)
		}

		f.Clear().Rowln().
			Success(color.BrightGreen("Successfully").Italic().String() + " imported bookmarks from git\n").
			Flush()

		return nil
	},
}

func init() {
	gitRemoteCmd.Flags().StringVar(&gitFlags.origin, "add", "", "git remote origin")
	_ = gitRemoteCmd.MarkFlagRequired("add")

	gitCloneCmd.Flags().StringVar(&gitFlags.repo, "repo", "", "repository URL")

	gitCmd.AddCommand(gitPullCmd, gitPushCmd, gitCommitCmd, gitRemoteCmd,
		gitInitCmd, gitLogCmd, gpgInitCmd, gitCloneCmd)
	rootCmd.AddCommand(gitCmd)
}
