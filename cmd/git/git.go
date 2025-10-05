package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// NewCmd is the git command.
func NewCmd() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:                "git",
		Short:              "Git commands",
		Aliases:            []string{"g"},
		PersistentPreRunE:  cli.HookEnsureGitEnv,
		RunE:               gitCommandFunc,
		DisableFlagParsing: true,
	}

	app := config.New()

	// git tracker
	gitTrackerCmd.Flags().SortFlags = false
	gitTrackerCmd.Flags().BoolVarP(&app.Flags.Status, "status", "s", false,
		"status tracked databases")
	gitTrackerCmd.Flags().BoolVarP(&app.Flags.Track, "track", "t", false,
		"track database in git")
	gitTrackerCmd.Flags().BoolVarP(&app.Flags.Untrack, "untrack", "u", false,
		"untrack database in git")
	gitTrackerCmd.Flags().BoolVarP(&app.Flags.Management, "manage", "m", false,
		"repos management in git")
	gitCmd.AddCommand(gitTrackerCmd)

	// git initializer
	initCmd.Flags().BoolVar(&app.Flags.Redo, "redo", false,
		"reinitialize")
	gitCmd.AddCommand(initCmd)

	// git import from repo
	ImportCmd.Flags().StringVarP(&app.Flags.Path, "uri", "i", "",
		"repo URI to import")
	gitCmd.AddCommand(ImportCmd) // public

	gitCmd.AddCommand(commitCmd, pushCmd, remoteCmd, rawCmd)
	gitCmd.AddCommand(testCmd)

	return gitCmd
}

var (
	// rawCmd proxies raw Git commands directly to the underlying git binary.
	rawCmd = &cobra.Command{
		Use:   "raw",
		Short: "raw git commands",
		RunE:  gitCommandFunc,
	}

	// initCmd initializes a new, empty Git repository.
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "create empty Git repository",
		RunE:  gitInitFunc,
	}

	// commitCmd records staged changes in the repository.
	commitCmd = &cobra.Command{
		Use:   "commit",
		Short: "record changes to the repository",
		RunE:  gitCommitFunc,
	}

	// ImportCmd clones a Git repository and imports bookmarks.
	ImportCmd = &cobra.Command{
		Use:   "import",
		Short: "import bookmarks from git",
		RunE:  cloneAndImport,
	}

	// pushCmd pushes local changes to the remote repository.
	pushCmd = &cobra.Command{
		Use:                "push",
		Short:              "push changes to the repository",
		DisableFlagParsing: true,
		RunE:               pushFunc,
	}

	// remoteCmd adds or updates the remote origin URL.
	remoteCmd = &cobra.Command{
		Use:   "remote",
		Short: "add remote origin",
		RunE:  remoteFunc,
	}
)

func gitCommitFunc(_ *cobra.Command, _ []string) error {
	app := config.New()
	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}

	r, err := db.New(gr.Loc.DBPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	defer r.Close()

	bs, err := r.All(context.Background())
	if err != nil {
		return fmt.Errorf("load records: %w", err)
	}

	if _, err := gr.Write(bs); err != nil {
		return err
	}

	return gr.Commit("update")
}

// cloneAndImport clones a git repo and imports its bookmarks.
func cloneAndImport(cmd *cobra.Command, args []string) error {
	app := config.New()
	if app.Flags.Path == "" {
		return git.ErrGitRepoURLEmpty
	}

	tmpPath := filepath.Join(os.TempDir(), app.Name+"-clone")
	if files.Exists(tmpPath) {
		_ = files.RemoveAll(tmpPath)
	}
	defer func() { _ = files.RemoveAll(tmpPath) }()

	c := ui.NewDefaultConsole(func(err error) {
		slog.Debug("cleaning up temp dir", "path", tmpPath)

		if err := files.RemoveAll(tmpPath); err != nil {
			slog.Error("cleaning up temp dir", "path", tmpPath)
		}

		sys.ErrAndExit(err)
	})

	// Set path with the temp dir
	gitCmd, err := sys.Which("git")
	if err != nil {
		return fmt.Errorf("%w: %q", err, "git")
	}

	gm := git.NewGit(tmpPath, git.WithCmd(gitCmd))
	imported, err := git.Import(c, gm, app)
	if err != nil {
		return err
	}
	if !git.IsInitialized(app.Git.Path) {
		slog.Warn("git import: repo not initialized", "path", app.Git.Path)
		return nil
	}

	for _, dbPath := range imported {
		gr, err := git.NewRepo(dbPath)
		if err != nil {
			return err
		}

		if gr.IsTracked() {
			if err := gr.Export(); err != nil {
				return err
			}
			if err := gr.Commit(cmd.Short); err != nil {
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

// gitInitFunc creates a new Git repository.
func gitInitFunc(_ *cobra.Command, _ []string) error {
	app := config.New()
	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}

	if err := gr.Git.Init(app.Flags.Redo); err != nil {
		return fmt.Errorf("init repo: %w", err)
	}

	c := ui.NewConsole(
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
		ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightBlue))),
	)

	if err := gr.AskForEncryption(c); err != nil {
		return err
	}

	if err := managementSelect(c, app); err != nil {
		return fmt.Errorf("select tracked: %w", err)
	}

	return nil
}

// gitCmd represents the git command.
func gitCommandFunc(command *cobra.Command, args []string) error {
	app := config.New()
	if !files.Exists(app.Git.Path) {
		return git.ErrGitNotInitialized
	}

	gitCmd, err := sys.Which("git")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(args) == 0 {
		args = append(args, "log", "--oneline")
	}

	g := git.NewGit(app.Git.Path, git.WithCmd(gitCmd))

	return g.Exec(args...)
}

func pushFunc(_ *cobra.Command, args []string) error {
	app := config.New()
	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}

	remote, err := gr.Git.Remote()
	if err != nil || remote == "" {
		return git.ErrGitNoRemote
	}

	proceed, err := gr.Git.HasUnpushedCommits()
	if err != nil {
		return err
	}

	if !proceed {
		return git.ErrGitUpToDate
	}

	sum, err := gr.SummaryUpdate(app.Info.Version)
	if err != nil {
		return err
	}

	sumFile := filepath.Join(gr.Loc.Path, git.SummaryFileName)
	if _, err := files.JSONWrite(sumFile, sum, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	if err := gr.Git.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	if err := gr.Git.Commit(fmt.Sprintf("[%s] update summary", gr.Loc.DBName)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	if err := gr.Git.Push(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func remoteFunc(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return git.ErrGitRepoURLEmpty
	}

	app := config.New()
	gr, err := git.NewRepo(app.DBPath)
	if err != nil {
		return err
	}

	if err := gr.Git.AddRemote(args[0], app.Flags.Force); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}

	sum, err := gr.SummaryUpdate(app.Info.Version)
	if err != nil {
		return err
	}

	sumFile := filepath.Join(gr.Loc.Path, git.SummaryFileName)
	if _, err := files.JSONWrite(sumFile, sum, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	if err := gr.Git.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	if err := gr.Git.Commit(fmt.Sprintf("[%s] update summary", gr.Loc.DBName)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return git.SetUpstream(app.Git.Path)
}

var testCmd = &cobra.Command{
	Use:    "test",
	Short:  "test git commands",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}
