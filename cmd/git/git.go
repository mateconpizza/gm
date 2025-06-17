//nolint:wrapcheck //ignore
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/locker/gpg"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

func init() {
	gitInitCmd.Flags().BoolVar(&gitFlags.redo, "redo", false, "reinitialize")
	gitCmd.AddCommand(gitCommitCmd, gitInitCmd, gitImportCmd, gitTrackerCmd, gitTestCmd)
	cmd.Root.AddCommand(gitCmd)
}

type gitFlagsType struct {
	redo bool
}

var (
	gitFlags = gitFlagsType{}

	gitCmd = &cobra.Command{
		Use:                "git",
		Short:              "git commands",
		Aliases:            []string{"g"},
		DisableFlagParsing: true,
		PersistentPreRunE:  ensureGitEnvironment,
		RunE:               gitCommandFunc,
	}

	gitInitCmd = &cobra.Command{
		Use:                "init",
		Short:              "create empty Git repository",
		DisableFlagParsing: false,
		RunE:               gitInitFunc,
	}

	gitCommitCmd = &cobra.Command{
		Use:                "commit",
		Short:              "record changes to the repository",
		DisableFlagParsing: false,
		RunE:               gitCommitFunc,
	}

	gitImportCmd = &cobra.Command{
		Use:                "import",
		Short:              "import bookmarks from git",
		DisableFlagParsing: false,
		RunE:               gitCloneAndImportFunc,
	}
)

func gitCommitFunc(_ *cobra.Command, _ []string) error {
	g, err := newGit(config.App.Path.Git)
	if err != nil {
		return err
	}
	gr := g.NewRepo(config.App.DBPath)
	g.Tracker.SetCurrent(gr)

	return handler.GitCommit(g, "Update")
}

// gitCloneAndImportFunc clones git repo and imports bookmarks.
func gitCloneAndImportFunc(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return git.ErrGitRepoURLEmpty
	}
	repoPathToClone := args[0]

	tmpPath := filepath.Join(os.TempDir(), config.App.Name+"-clone")
	go func() {
		_ = files.RemoveAll(tmpPath)
	}()

	f := frame.New(frame.WithColorBorder(color.Gray))
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		fmt.Println("cleaning temp files...")
		_ = files.RemoveAll(tmpPath)
		sys.ErrAndExit(err)
	}))

	g, err := newGit(config.App.Path.Git)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	imported, err := port.GitImport(t, f, tmpPath, repoPathToClone)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if !g.IsInitialized() {
		return nil
	}
	if err := g.Tracker.Load(); err != nil {
		return fmt.Errorf("%w", err)
	}

	for _, dbPath := range imported {
		gr := g.NewRepo(dbPath)
		g.Tracker.SetCurrent(gr)
		if err := trackExportCommit(t, f, g); err != nil {
			return err
		}
	}

	return nil
}

// newGit returns a new git manager.
func newGit(repoPath string) (*git.Manager, error) {
	gCmd := "git"
	gitCmd, err := sys.Which(gCmd)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, gCmd)
	}

	return git.New(repoPath, git.WithCmd(gitCmd)), nil
}

// gitInitFunc creates a new Git repository.
func gitInitFunc(_ *cobra.Command, _ []string) error {
	t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
	f := frame.New(frame.WithColorBorder(color.BrightBlue))
	g, err := newGit(config.App.Path.Git)
	if err != nil {
		return err
	}
	g.Tracker.SetCurrent(g.NewRepo(config.App.DBPath))

	if err := g.Init(gitFlags.redo); err != nil {
		return fmt.Errorf("init repo: %w", err)
	}

	if err := g.Tracker.Load(); err != nil {
		return fmt.Errorf("%w", err)
	}

	tracked, err := managementSelect(t, f, g)
	if err != nil {
		return fmt.Errorf("select tracked: %w", err)
	}

	if len(tracked) == 0 {
		return git.ErrGitNoTrackedRepos
	}

	if t.Confirm(f.Reset().Question("Use GPG for encryption?").String(), "y") {
		if err := gpg.Init(g.RepoPath); err != nil {
			return fmt.Errorf("gpg init: %w", err)
		}
		if err := g.AddAll(); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
		if err := g.Commit("GPG repo initialized"); err != nil {
			return fmt.Errorf("git commit: %w", err)
		}
	}

	for _, dbPath := range tracked {
		gr := g.NewRepo(dbPath)
		if err := g.Tracker.Track(gr).Save(); err != nil {
			return fmt.Errorf("%w", err)
		}
		g.Tracker.SetCurrent(gr)

		if err := initTracking(g); err != nil {
			return err
		}
	}

	return nil
}

// ensureGitEnvironment checks if the environment is ready for git commands.
func ensureGitEnvironment(command *cobra.Command, _ []string) error {
	if err := handler.AssertDefaultDatabaseExists(); err != nil {
		return fmt.Errorf("%w", err)
	}

	gitCmd, err := sys.Which("git")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	g := git.New(config.App.Path.Git, git.WithCmd(gitCmd))

	switch command.Name() {
	case "init", "import":
		return nil
	}

	if !g.IsInitialized() {
		return git.ErrGitNotInitialized
	}

	return nil
}

// gitCmd represents the git command.
func gitCommandFunc(command *cobra.Command, args []string) error {
	if slices.ContainsFunc([]string{"-h", "--help", "help"}, func(x string) bool {
		return slices.Contains(args, x)
	}) {
		return command.Help()
	}

	gitCmd, err := sys.Which("git")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	g := git.New(config.App.Path.Git, git.WithCmd(gitCmd))

	if len(args) == 0 {
		args = append(args, "log", "--oneline")
	}

	return g.Exec(args...)
}

var gitTestCmd = &cobra.Command{
	Use:                "test",
	Short:              "Test git commands",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitCmd, err := sys.Which("git")
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		g := git.New(config.App.Path.Git, git.WithCmd(gitCmd))

		if len(args) == 0 {
			args = append(args, "log", "--oneline")
		}

		return g.Exec(args...)
	},
}
