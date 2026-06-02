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

func Initialized(root string) bool {
	return fileExists(root)
}

type GitOpt func(*GitOptions)

type GitOptions struct {
	w io.Writer

	logHook CmdLogger
}

func WithGitWriter(w io.Writer) GitOpt {
	return func(o *GitOptions) {
		o.w = w
	}
}

func WithGitCommandLogger(hook CmdLogger) GitOpt {
	return func(o *GitOptions) {
		o.logHook = hook
	}
}

type CommitCfg struct {
	gr  *Repo
	ver string
	msg string
}

func NewCommitCfg(gr *Repo, ver, msg string) *CommitCfg {
	return &CommitCfg{
		gr:  gr,
		ver: ver,
		msg: msg,
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
func (g *Git) Push(ctx context.Context) error               { return push(ctx, g.fullpath) }
func (g *Git) Commit(ctx context.Context, msg string) error { return Commit(ctx, g.fullpath, msg) }

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

func Command() (string, error) { return which(command) }

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

	logCmd := g.logHook
	if logCmd == nil {
		logCmd = func(commands []string) { fmt.Println(strings.Join(commands, " ")) }
	}

	logCmd(commands)
	return execCmdWithWriter(ctx, g.w, nil, commands...)
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

	if o.w == nil {
		o.w = os.Stdout
	}

	return &Git{
		fullpath:   path,
		bin:        binPath,
		GitOptions: o,
	}, nil
}
