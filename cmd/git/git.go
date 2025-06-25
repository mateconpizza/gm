//nolint:wrapcheck //ignore
package git

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

func init() {
	gitInitCmd.Flags().BoolVar(&gitFlags.redo, "redo", false, "reinitialize")
	gitCmd.AddCommand(GitImportCmd) // public
	gitCmd.AddCommand(gitCommitCmd, gitInitCmd, gitTrackerCmd, gitPushCmd, gitRemoteCmd, gitRawCmd)
	cmd.Root.AddCommand(gitCmd)
}

type gitFlagsType struct {
	redo bool
}

var (
	gitFlags = gitFlagsType{}

	gitCmd = &cobra.Command{
		Use:                "git",
		Short:              "Git commands",
		Aliases:            []string{"g"},
		DisableFlagParsing: true,
		PersistentPreRunE:  ensureGitEnvironment,
		RunE:               gitCommandFunc,
	}

	gitRawCmd = &cobra.Command{
		Use:                "raw",
		Short:              "raw git commands",
		DisableFlagParsing: true,
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

	GitImportCmd = &cobra.Command{
		Use:                "import",
		Short:              "import bookmarks from git",
		DisableFlagParsing: false,
		RunE:               gitCloneAndImportFunc,
	}

	gitPushCmd = &cobra.Command{
		Use:                "push",
		Short:              "push changes to the repository",
		DisableFlagParsing: true,
		RunE:               gitPushFunc,
	}

	gitRemoteCmd = &cobra.Command{
		Use:                "remote",
		Short:              "add remote origin",
		DisableFlagParsing: false,
		RunE:               gitRemoteFunc,
	}
)

func gitCommitFunc(_ *cobra.Command, _ []string) error {
	g, err := handler.NewGit(config.App.Path.Git)
	if err != nil {
		return err
	}
	g.Tracker.SetCurrent(g.NewRepo(config.App.DBPath))

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

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			slog.Debug("cleaning up temp dir", "path", tmpPath)
			_ = files.RemoveAll(tmpPath)

			sys.ErrAndExit(err)
		}))),
	)

	// Set path with the temp dir
	gm, err := handler.NewGit(tmpPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	imported, err := port.GitImport(c, gm, repoPathToClone)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Update with the default repo path
	gm.SetRepoPath(config.App.Path.Git)

	if !gm.IsInitialized() {
		slog.Warn("git import: repo not initialized", "path", config.App.Path.Git)
		return nil
	}

	if err := gm.Tracker.Load(); err != nil {
		return fmt.Errorf("%w", err)
	}

	for _, dbPath := range imported {
		gm.Tracker.SetCurrent(gm.NewRepo(dbPath))
		if err := handler.GitTrackExportCommit(c, gm, "import from git"); err != nil {
			return err
		}
	}

	return nil
}

// gitInitFunc creates a new Git repository.
func gitInitFunc(_ *cobra.Command, _ []string) error {
	g, err := handler.NewGit(config.App.Path.Git)
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

	c := ui.NewConsole(
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
		ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightBlue))),
	)
	tracked, err := managementSelect(c, g)
	if err != nil {
		return fmt.Errorf("select tracked: %w", err)
	}

	if len(tracked) == 0 {
		return git.ErrGitNoRepos
	}

	if c.Confirm("Use GPG for encryption?", "n") {
		if err := gpg.Init(g.RepoPath, git.AttributesFile); err != nil {
			return fmt.Errorf("gpg init: %w", err)
		}
		// add diff to git config
		for k, v := range gpg.GitDiffConf {
			if err := g.SetConfigLocal(k, strings.Join(v, " ")); err != nil {
				return err
			}
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

		if err := initTracking(c, g); err != nil {
			return err
		}
	}

	return nil
}

// ensureGitEnvironment checks if the environment is ready for git commands.
func ensureGitEnvironment(command *cobra.Command, args []string) error {
	if err := cmd.RequireDatabase(command, args); err != nil {
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

	if len(args) == 0 {
		args = append(args, "log", "--oneline")
	}

	g := git.New(config.App.Path.Git, git.WithCmd(gitCmd))

	return g.Exec(args...)
}

func gitPushFunc(_ *cobra.Command, args []string) error {
	g, err := handler.NewGit(config.App.Path.Git)
	if err != nil {
		return err
	}

	gr := g.NewRepo(config.App.DBPath)
	g.Tracker.SetCurrent(gr)

	remote, err := g.Remote()
	if err != nil || remote == "" {
		return git.ErrGitNoRemote
	}

	proceed, err := g.HasUnpushedCommits()
	if err != nil {
		return err
	}

	if !proceed {
		return git.ErrGitUpToDate
	}

	sum, err := handler.GitSummary(g, config.App.Info.Version)
	if err != nil {
		return err
	}

	sumFile := filepath.Join(g.Tracker.Current().Path, git.SummaryFileName)
	if err := files.JSONWrite(sumFile, sum, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	if err := g.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	if err := g.Commit(fmt.Sprintf("[%s] Update summary", gr.DBName)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	if err := g.Push(); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func gitRemoteFunc(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return git.ErrGitRepoURLEmpty
	}

	cfg := config.App
	g, err := handler.NewGit(cfg.Path.Git)
	if err != nil {
		return err
	}

	gr := g.NewRepo(config.App.DBPath)
	g.Tracker.SetCurrent(gr)

	if err := g.AddRemote(args[0]); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}

	sum, err := handler.GitSummary(g, config.App.Info.Version)
	if err != nil {
		return err
	}

	sumFile := filepath.Join(g.Tracker.Current().Path, git.SummaryFileName)
	if err := files.JSONWrite(sumFile, sum, true); err != nil {
		return fmt.Errorf("writing summary: %w", err)
	}

	if err := g.AddAll(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	if err := g.Commit(fmt.Sprintf("[%s] Update summary", gr.DBName)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return git.SetUpstream(cfg.Path.Git)
}
