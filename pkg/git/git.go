// Parkage git provides high-level utilities to initialize, manage, and
// interact with the bookmark's Git repositorie.
package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	command        = "git"
	AttributesFile = ".gitattributes"
)

type CmdLogger func(w io.Writer, commands []string)

type ExecuterFunc func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error

type GitOpt func(*GitOptions)

type GitOptions struct {
	writer        io.Writer
	commandLogger CmdLogger
	executer      ExecuterFunc
	cmd           *Commander
}

func WithGitWriter(w io.Writer) GitOpt {
	return func(o *GitOptions) {
		o.writer = w
	}
}

func WithGitCommandLogger(hook CmdLogger) GitOpt {
	return func(o *GitOptions) {
		o.commandLogger = hook
	}
}

func WithExecuter(fn ExecuterFunc) GitOpt {
	return func(o *GitOptions) {
		o.executer = fn
	}
}

func WithCommander(c *Commander) GitOpt {
	return func(o *GitOptions) {
		o.cmd = c
	}
}

// Git handles operational tasks on a local Git repository.
type Git struct {
	*GitOptions

	fullpath string
}

// New verifies the system environment and returns a usable Git workflow
// client.
func New(path string, opts ...GitOpt) (*Git, error) {
	o := &GitOptions{writer: os.Stdout}
	for _, opt := range opts {
		opt(o)
	}

	if o.commandLogger == nil {
		o.commandLogger = func(w io.Writer, commands []string) {
			fmt.Fprintln(o.writer, strings.Join(commands, " "))
		}
	}

	bin := command
	if o.executer == nil {
		o.executer = defaultExecuter

		found, err := which(command)
		if err != nil {
			return nil, fmt.Errorf("%w: %q", err, command)
		}
		bin = found
	}

	o.cmd = NewCommander(bin).
		WithExecutor(o.executer)

	return &Git{
		fullpath:   path,
		GitOptions: o,
	}, nil
}

func Cmd() (string, error)                      { return which(command) }
func Initialized(root string) bool              { return fileExists(root) }
func (g *Git) Root() string                     { return g.fullpath }
func (g *Git) Writer() io.Writer                { return g.writer }
func (g *Git) Bin() string                      { return g.cmd.bin }
func (g *Git) AddAll(ctx context.Context) error { return g.run(ctx, g.fullpath, "add", ".") }

func (g *Git) Commit(ctx context.Context, msg string) error {
	return g.run(ctx, g.fullpath, "commit", "-m", msg)
}

func (g *Git) Exec(ctx context.Context, commands ...string) error {
	return g.run(ctx, g.fullpath, commands...)
}

func (g *Git) SetCfgLocal(ctx context.Context, k, v string) error {
	return g.run(ctx, g.fullpath, "config", "--local", k, v)
}

func (g *Git) CloneInto(ctx context.Context, srcURL, destPath string) error {
	return g.run(ctx, "", "clone", srcURL, destPath)
}

// Branch returns the current branch.
func (g *Git) Branch(ctx context.Context) (string, error) {
	return g.cmd.Output(ctx, g.fullpath, "rev-parse", "--abbrev-ref", "HEAD")
}

func (g *Git) Remote(ctx context.Context) (string, error) {
	return g.cmd.Output(ctx, g.fullpath, "config", "--get", "remote.origin.url")
}

// Status returns the status of the repo.
func (g *Git) Status(ctx context.Context) (string, error) {
	if !g.hasCommits(ctx) {
		return "", ErrGitNoCommits
	}

	added, modified, deleted, err := g.countStagedChanges(ctx)
	if err != nil {
		return "", err
	}

	return formatStatus(added, modified, deleted), nil
}

// HasUnpulledCommits checks if there are commits on the upstream
// branch that have not yet been pulled locally.
func (g *Git) HasUnpulledCommits(ctx context.Context) (bool, error) {
	if err := g.HasUpstream(ctx); err != nil {
		return false, err
	}

	// Count commits present in the upstream but not locally
	out, err := g.cmd.Output(ctx, g.fullpath, "rev-list", "--count", "@{u}", "^HEAD")
	if err != nil {
		return false, fmt.Errorf("checking unpulled commits: %w", err)
	}

	return strings.TrimSpace(out) != "0", nil
}

// HasUpstream checks whether the current branch has an upstream (remote tracking branch) configured.
func (g *Git) HasUpstream(ctx context.Context) error {
	err := g.cmd.Run(ctx, io.Discard, g.fullpath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return ErrGitNoUpstream
	}

	return nil
}

// HasChanges checks if there are any staged or unstaged changes in the repo.
func (g *Git) HasChanges(ctx context.Context) (bool, error) {
	output, err := g.cmd.Output(ctx, g.fullpath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return strings.TrimSpace(output) != "", nil
}

func (g *Git) FileHasChanges(ctx context.Context, filePath string) bool {
	_, err := g.cmd.Output(ctx, g.fullpath, "diff", "--quiet", "--", filePath)
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

// HasUnpushedCommits checks if there are any unpushed commits.
func (g *Git) HasUnpushedCommits(ctx context.Context) (bool, error) {
	n, err := g.unpushedCommitsCount(ctx)
	if err != nil {
		return false, fmt.Errorf("count unpushed commits: %w", err)
	}

	return n != 0, nil
}

// func (g *Git) Clone(ctx context.Context, repoURL string) error {
// 	return g.run(ctx, "", "clone", repoURL, g.fullpath)
// }

func (g *Git) UnpushedCommits(ctx context.Context) (int, error) {
	if err := g.HasUpstream(ctx); err != nil {
		return 0, err
	}
	return g.unpushedCommitsCount(ctx)
}

func (g *Git) Push(ctx context.Context) error {
	// check if remote exists
	remotes, err := g.cmd.Output(ctx, g.fullpath, "remote")
	if err != nil {
		return fmt.Errorf("git remote check failed: %w", err)
	}

	if strings.TrimSpace(remotes) == "" {
		return ErrGitNoUpstream
	}

	branch, err := g.Branch(ctx)
	if err != nil {
		return fmt.Errorf("could not get current branch: %w", err)
	}

	// check if branch has upstream
	err = g.cmd.Run(ctx, io.Discard, g.fullpath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		// no upstream, so set it
		return g.cmd.Run(ctx, os.Stdout, g.fullpath, "push", "--set-upstream", "origin", branch)
	}

	return g.run(ctx, g.fullpath, "push")
}

// Init creates a new Git repository.
func (g *Git) Init(ctx context.Context, force bool) error {
	p := g.fullpath
	if IsInitialized(p) && !force {
		return ErrGitInitialized
	}

	if fileExists(p) && force {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(p, DirPerm); err != nil {
		return err
	}

	return g.run(ctx, p, "init")
}

// AddRemote adds a remote repository.
func (g *Git) AddRemote(ctx context.Context, repoURL string, force bool) error {
	action := "add"
	if force {
		action = "set-url"
	}

	return g.run(ctx, g.fullpath, "remote", action, "origin", repoURL)
}

func (g *Git) SetUpstream(ctx context.Context, repoPath string) error {
	err := g.HasUpstream(ctx)
	if err == nil {
		return ErrGitUpstreamExists
	}
	b, err := g.Branch(ctx)
	if err != nil {
		return err
	}

	return g.run(ctx, repoPath, "push", "--set-upstream", "origin", b)
}

func (g *Git) run(ctx context.Context, repoPath string, commands ...string) error {
	g.commandLogger(g.writer, commands)
	return g.cmd.Exec(ctx, g.writer, repoPath, commands...)
}

// hasCommits checks if the repo has commits.
func (g *Git) hasCommits(ctx context.Context) bool {
	err := g.cmd.Run(ctx, io.Discard, g.fullpath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return false
		}

		return false
	}

	return true
}

func (g *Git) unpushedCommitsCount(ctx context.Context) (int, error) {
	s, err := g.cmd.Output(ctx, g.fullpath, "rev-list", "--count", "HEAD", "^@{u}")
	if err != nil {
		return 0, fmt.Errorf("count unpushed commits: %w", err)
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parse unpushed commit count %q: %w", s, err)
	}

	return n, nil
}

func (g *Git) countStagedChanges(ctx context.Context) (added, modified, deleted int, err error) {
	out, err := g.cmd.Output(ctx, g.fullpath, "diff", "--cached", "--name-status")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to run git diff-tree: %w", err)
	}

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || filepath.Base(fields[1]) == SummaryFileName {
			continue
		}

		switch fields[0] {
		case "A":
			added++
		case "M":
			modified++
		case "D":
			deleted++
		}
	}
	return added, modified, deleted, nil
}
