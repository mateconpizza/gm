// Package git provides high-level utilities to initialize, manage, and
// interact with the bookmark's Git repositorie.
package git

import (
	"context"
	"fmt"
)

const (
	gitCmd         = "git"
	AttributesFile = ".gitattributes"
)

type GitOptFn func(*GitOpts)

type GitOpts struct {
	cmd string
	ctx context.Context
}

// Git handles operational tasks on a local Git repository.
type Git struct {
	GitOpts
	repoPath      string
	isInitialized bool
}

func defaultOpts() *GitOpts {
	return &GitOpts{
		cmd: "git",
	}
}

// WithCmd sets the Git command to use.
func WithCmd(cmd string) GitOptFn {
	return func(o *GitOpts) {
		o.cmd = cmd
	}
}

func WithContext(ctx context.Context) GitOptFn {
	return func(o *GitOpts) {
		o.ctx = ctx
	}
}

// IsInitialized returns true if the Git repository is initialized.
func (g *Git) IsInitialized() bool {
	if !g.isInitialized {
		g.isInitialized = IsInitialized(g.repoPath)
	}

	return g.isInitialized
}

// Init creates a new Git repository.
func (g *Git) Init(force bool) error {
	return initialize(g.ctx, g.repoPath, force)
}

// FullPath returns the absolute directory path where the Git repository lives.
func (g *Git) FullPath() string {
	return g.repoPath
}

// branch returns the name of the current branch.
func (g *Git) branch() (string, error) {
	return branch(g.ctx, g.repoPath)
}

// Remote returns the origin of the repository.
func (g *Git) Remote() (string, error) {
	return Remote(g.ctx, g.repoPath)
}

// status returns the status of the repository.
func (g *Git) status() (string, error) {
	return status(g.ctx, g.repoPath)
}

// HasUnpushedCommits checks if there are any unpushed commits in the repo.
func (g *Git) HasUnpushedCommits() (bool, error) {
	return hasUnpushedCommits(g.ctx, g.repoPath)
}

// hasChanges checks if there are any staged or unstaged changes in the repo.
func (g *Git) hasChanges() (bool, error) {
	return hasChanges(g.ctx, g.repoPath)
}

// AddAll adds all local changes.
func (g *Git) AddAll() error {
	return addAll(g.ctx, g.repoPath)
}

// AddRemote adds a remote repository.
func (g *Git) AddRemote(repoURL string, force bool) error {
	return addRemote(g.ctx, g.repoPath, repoURL, force)
}

// Clone clones a repository.
func (g *Git) Clone(repoURL string) error {
	return cloneRepo(g.ctx, g.repoPath, repoURL)
}

func (g *Git) CloneInto(repoURL, destPath string) error {
	return cloneRepo(g.ctx, destPath, repoURL)
}

// Push pushes changes to the remote repository.
func (g *Git) Push() error {
	return push(g.ctx, g.repoPath)
}

// Commit commits changes to the repository.
func (g *Git) Commit(msg string) error {
	return Commit(g.ctx, g.repoPath, msg)
}

// SetRepoPath sets the repository path.
func (g *Git) SetRepoPath(path string) {
	g.isInitialized = false
	g.repoPath = path
}

// setConfigLocal sets a local config value.
func (g *Git) setConfigLocal(k, v string) error {
	return setConfigLocal(g.ctx, g.repoPath, k, v)
}

// Exec executes a command in the repository.
func (g *Git) Exec(commands ...string) error {
	return runGitCmd(g.ctx, g.repoPath, commands...)
}

// newGit creates a base Git wrapper instance with options.
func newGit(path string, opts ...GitOptFn) *Git {
	o := defaultOpts()
	for _, fn := range opts {
		fn(o)
	}

	// Ensure context always exists
	if o.ctx == nil {
		o.ctx = context.Background()
	}

	return &Git{
		repoPath: path,
		GitOpts:  *o,
	}
}

// New verifies the system environment and returns a usable Git workflow
// client.
func New(ctx context.Context, path string) (*Git, error) {
	gitCmd, err := which("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, "git")
	}

	g := newGit(path, WithCmd(gitCmd), WithContext(ctx))
	return g, nil
}
