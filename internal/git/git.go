// Package git provides high-level utilities to initialize, manage, and
// interact with the bookmark's Git repositorie, including commit, push, clone,
// tracking, and status operations.
package git

const gitCmd = "git"

type GitOptFn func(*GitOpts)

type GitOpts struct {
	cmd string
}

// Manager represents a Git repository manager.
type Manager struct {
	GitOpts
	RepoPath      string
	Tracker       *Tracker
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

// IsInitialized returns true if the Git repository is initialized.
func (gm *Manager) IsInitialized() bool {
	if !gm.isInitialized {
		gm.isInitialized = IsInitialized(gm.RepoPath)
	}

	return gm.isInitialized
}

// Init creates a new Git repository.
func (gm *Manager) Init(force bool) error {
	return initialize(gm.RepoPath, force)
}

// Branch returns the name of the current branch.
func (gm *Manager) Branch() (string, error) {
	return branch(gm.RepoPath)
}

// Remote returns the origin of the repository.
func (gm *Manager) Remote() (string, error) {
	return remote(gm.RepoPath)
}

// Status returns the status of the repository.
func (gm *Manager) Status() (string, error) {
	return status(gm.RepoPath)
}

func (gm *Manager) HasUnpushedCommits() (bool, error) {
	return hasUnpushedCommits(gm.RepoPath)
}

// HasChanges checks if there are any staged or unstaged changes in the repo.
func (gm *Manager) HasChanges() (bool, error) {
	return hasChanges(gm.RepoPath)
}

// AddAll adds all local changes.
func (gm *Manager) AddAll() error {
	return addAll(gm.RepoPath)
}

// AddRemote adds a remote repository.
func (gm *Manager) AddRemote(repoURL string) error {
	return addRemote(gm.RepoPath, repoURL)
}

// Clone clones a repository.
func (gm *Manager) Clone(repoURL string) error {
	return Clone(gm.RepoPath, repoURL)
}

// Push pushes changes to the remote repository.
func (gm *Manager) Push() error {
	return push(gm.RepoPath)
}

// Commit commits changes to the repository.
func (gm *Manager) Commit(msg string) error {
	return commitChanges(gm.RepoPath, msg)
}

// SetRepoPath sets the repository path.
func (gm *Manager) SetRepoPath(repoPath string) {
	gm.isInitialized = false
	gm.RepoPath = repoPath
}

// Exec executes a command in the repository.
func (gm *Manager) Exec(commands ...string) error {
	return runGitCmd(gm.RepoPath, commands...)
}

// NewRepo creates a new Git repository.
func (gm *Manager) NewRepo(dbPath string) *GitRepository {
	return newGitRepository(gm.RepoPath, dbPath)
}

// New creates a new GitManager.
func New(repoPath string, opts ...GitOptFn) *Manager {
	o := defaultOpts()
	for _, fn := range opts {
		fn(o)
	}

	return &Manager{
		RepoPath: repoPath,
		GitOpts:  *o,
		Tracker:  NewTracker(repoPath),
	}
}
