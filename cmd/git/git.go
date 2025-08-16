//nolint:wrapcheck //ignore
package git

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func init() {
	gitInitCmd.Flags().BoolVar(&gitFlags.redo, "redo", false, "reinitialize")
	gitCmd.AddCommand(GitImportCmd) // public
	gitCmd.AddCommand(gitCommitCmd, gitInitCmd, gitTrackerCmd, gitPushCmd, gitRemoteCmd, gitRawCmd)
	gitCmd.AddCommand(gitTestCmd)
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
	gr, err := git.NewRepo(config.App.DBPath)
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

// gitCloneAndImportFunc clones a git repo and imports its bookmarks.
func gitCloneAndImportFunc(command *cobra.Command, args []string) error {
	if len(args) == 0 {
		return git.ErrGitRepoURLEmpty
	}
	repoPathToClone := args[0]
	tmpPath := filepath.Join(os.TempDir(), config.App.Name+"-clone")
	if files.Exists(tmpPath) {
		_ = files.RemoveAll(tmpPath)
	}
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
	gitCmd, err := sys.Which("git")
	if err != nil {
		return fmt.Errorf("%w: %q", err, "git")
	}

	gm := git.NewGit(tmpPath, git.WithCmd(gitCmd))
	imported, err := git.Import(c, gm, repoPathToClone)
	if err != nil {
		return err
	}
	if !git.IsInitialized(config.App.Git.Path) {
		slog.Warn("git import: repo not initialized", "path", config.App.Git.Path)
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
			if err := gr.Commit(command.Short); err != nil {
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
	gr, err := git.NewRepo(config.App.DBPath)
	if err != nil {
		return err
	}

	if err := gr.Git.Init(gitFlags.redo); err != nil {
		return fmt.Errorf("init repo: %w", err)
	}

	c := ui.NewConsole(
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
		ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightBlue))),
	)

	if err := gr.AskForEncryption(c); err != nil {
		return err
	}

	if err := managementSelect(c); err != nil {
		return fmt.Errorf("select tracked: %w", err)
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

	gm := git.NewGit(config.App.Git.Path, git.WithCmd(gitCmd))

	switch command.Name() {
	case "init", "import":
		return nil
	}

	if !gm.IsInitialized() {
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

	g := git.NewGit(config.App.Git.Path, git.WithCmd(gitCmd))

	return g.Exec(args...)
}

func gitPushFunc(_ *cobra.Command, args []string) error {
	gr, err := git.NewRepo(config.App.DBPath)
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

	sum, err := gr.SummaryUpdate()
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

func gitRemoteFunc(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return git.ErrGitRepoURLEmpty
	}

	cfg := config.App
	gr, err := git.NewRepo(config.App.DBPath)
	if err != nil {
		return err
	}

	if err := gr.Git.AddRemote(args[0]); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}

	sum, err := gr.SummaryUpdate()
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

	return git.SetUpstream(cfg.Git.Path)
}

var gitTestCmd = &cobra.Command{
	Use:    "test",
	Short:  "test git commands",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := db.New(config.App.DBPath)
		if err != nil {
			return err
		}
		defer r.Close()

		bs, err := r.All(context.Background())
		if err != nil {
			return err
		}

		b := bs[0]
		te, err := files.NewEditor(config.App.Env.Editor)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		c := ui.NewConsole(
			ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))),
			ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightBlue))),
		)

		oldB := b.Bytes()
		for {
			newB, err := te.EditBytes(oldB, "json")
			if err != nil {
				return err
			}
			if bytes.Equal(newB, oldB) {
				fmt.Println("no changes")
				return nil
			}

			oldB = bytes.TrimRight(oldB, "\n")
			newB = bytes.TrimRight(newB, "\n")

			diff := txt.Diff(oldB, newB)
			fmt.Println(txt.DiffColor(diff))

			opt, err := c.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
			if err != nil {
				return fmt.Errorf("choose: %w", err)
			}

			switch strings.ToLower(opt) {
			case "y", "yes":
				bm, err := bookmark.NewFromBuffer(newB)
				if err != nil {
					return err
				}

				if err := r.Update(context.Background(), bm, b); err != nil {
					return fmt.Errorf("update: %w", err)
				}

				return nil
			case "n", "no":
				return sys.ErrActionAborted
			case "e", "edit":
				oldB = newB
			}
		}
	},
}
