// Parkage git provides high-level utilities to initialize, manage, and
// interact with the bookmark's Git repositorie.
package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	command        = "git"
	AttributesFile = ".gitattributes"
)

type CmdLogger func(w io.Writer, commands []string)

type GitOpt func(*GitOptions)

type GitOptions struct {
	w             io.Writer
	commandLogger CmdLogger
}

func WithGitWriter(w io.Writer) GitOpt {
	return func(o *GitOptions) {
		o.w = w
	}
}

func WithGitCommandLogger(hook CmdLogger) GitOpt {
	return func(o *GitOptions) {
		o.commandLogger = hook
	}
}

// Git handles operational tasks on a local Git repository.
type Git struct {
	bin      string
	fullpath string
	*GitOptions
}

func (g *Git) Root() string                                 { return g.fullpath }
func (g *Git) Writer() io.Writer                            { return g.w }
func (g *Git) Bin() string                                  { return g.bin }
func (g *Git) Branch(ctx context.Context) (string, error)   { return branch(ctx, g.fullpath) }
func (g *Git) Remote(ctx context.Context) (string, error)   { return Remote(ctx, g.fullpath) }
func (g *Git) Status(ctx context.Context) (string, error)   { return status(ctx, g.fullpath) }
func (g *Git) HasChanges(ctx context.Context) (bool, error) { return HasChanges(ctx, g.fullpath) }
func (g *Git) AddAll(ctx context.Context) error             { return g.run(ctx, g.fullpath, "add", ".") }
func (g *Git) Push(ctx context.Context) error               { return g.doPush(ctx) }

func (g *Git) Commit(ctx context.Context, msg string) error {
	return g.run(ctx, g.fullpath, "commit", "-m", msg)
}

func (g *Git) Exec(ctx context.Context, commands ...string) error {
	return g.run(ctx, g.fullpath, commands...)
}

func (g *Git) HasUnpushedCommits(ctx context.Context) (bool, error) {
	return hasUnpushedCommits(ctx, g.fullpath)
}

func (g *Git) Clone(ctx context.Context, repoURL string) error {
	return g.run(ctx, "", "clone", repoURL, g.fullpath)
}

func (g *Git) SetCfgLocal(ctx context.Context, k, v string) error {
	return g.run(ctx, g.fullpath, "config", "--local", k, v)
}

func (g *Git) CloneInto(ctx context.Context, repoURL, destPath string) error {
	return g.run(ctx, "", "clone", repoURL, destPath)
}

func (g *Git) UnpushedCommits(ctx context.Context) (int, error) {
	if err := HasUpstream(ctx, g.fullpath); err != nil {
		return 0, err
	}
	return unpushedCommitsCount(ctx, g.fullpath)
}

func Cmd() (string, error)         { return which(command) }
func Initialized(root string) bool { return fileExists(root) }

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
	err := HasUpstream(ctx, repoPath)
	if err == nil {
		return ErrGitUpstreamExists
	}
	b, err := branch(ctx, repoPath)
	if err != nil {
		return err
	}

	return g.run(ctx, repoPath, "push", "--set-upstream", "origin", b)
}

func (g *Git) run(ctx context.Context, repoPath string, commands ...string) error {
	cmd := []string{g.bin}
	if repoPath != "" {
		cmd = append(cmd, "-C", repoPath)
	}
	commands = append(cmd, commands...)

	logCmd := g.commandLogger
	if logCmd == nil {
		logCmd = func(w io.Writer, commands []string) {
			fmt.Fprintln(g.w, strings.Join(commands, " "))
		}
	}

	logCmd(g.w, commands)
	return execCmdWithWriter(ctx, g.w, nil, commands...)
}

func (g *Git) doPush(ctx context.Context) error {
	// check if remote exists
	remotes, err := runWithOutput(ctx, g.fullpath, "remote")
	if err != nil {
		return fmt.Errorf("git remote check failed: %w", err)
	}

	if strings.TrimSpace(remotes) == "" {
		return ErrGitNoUpstream
	}

	branch, err := branch(ctx, g.fullpath)
	if err != nil {
		return fmt.Errorf("could not get current branch: %w", err)
	}

	// check if branch has upstream
	err = runWithWriter(ctx, io.Discard, g.fullpath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		// no upstream, so set it
		return runWithWriter(ctx, os.Stdout, g.fullpath, "push", "--set-upstream", "origin", branch)
	}

	return g.run(ctx, g.fullpath, "push")
}

func Run(ctx context.Context, repoPath string, commands ...string) error {
	g, err := New(repoPath)
	if err != nil {
		return err
	}

	return g.run(ctx, repoPath, commands...)
}

// New verifies the system environment and returns a usable Git workflow
// client.
func New(path string, opts ...GitOpt) (*Git, error) {
	o := &GitOptions{}
	for _, opt := range opts {
		opt(o)
	}

	binPath, err := which(command)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, command)
	}

	return &Git{
		fullpath:   path,
		bin:        binPath,
		GitOptions: o,
	}, nil
}
