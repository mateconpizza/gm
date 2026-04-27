package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrGitInitialized     = errors.New("git: is initialized")
	ErrGitNotInitialized  = errors.New("git: is not initialized")
	ErrGitNoCommits       = errors.New("git: no commits found")
	ErrGitNoUpstream      = errors.New("git: no upstream configured")
	ErrGitUpstreamExists  = errors.New("git: remote origin already exists")
	ErrGitNothingToCommit = errors.New("git: nothing to commit, working tree clean")
	ErrGitUpToDate        = errors.New("git: everything up-to-date")
	ErrGitRepoNotFound    = errors.New("git: repo not found")
	ErrGitRepoURLEmpty    = errors.New("git: repo url is empty")
)

func cloneRepo(ctx context.Context, destRepoPath, repoURL string) error {
	return runGitCmd(ctx, "", "clone", repoURL, destRepoPath)
}

// addAll adds all local changes.
func addAll(ctx context.Context, repoPath string) error {
	return runGitCmd(ctx, repoPath, "add", ".")
}

// addRemote adds a remote repository.
func addRemote(ctx context.Context, repoPath, repoURL string, force bool) error {
	if force {
		return runGitCmd(ctx, repoPath, "remote", "set-url", "origin", repoURL)
	}

	return runGitCmd(ctx, repoPath, "remote", "add", "origin", repoURL)
}

// SetUpstream sets the upstream for the current branch.
func SetUpstream(ctx context.Context, repoPath string) error {
	err := HasUpstream(ctx, repoPath)
	if err == nil {
		return ErrGitUpstreamExists
	}

	b, err := branch(ctx, repoPath)
	if err != nil {
		return err
	}

	return runGitCmd(ctx, repoPath, "push", "--set-upstream", "origin", b)
}

// hasUnpushedCommits checks if there are any unpushed commits.
func hasUnpushedCommits(ctx context.Context, repoPath string) (bool, error) {
	s, err := runWithOutput(ctx, repoPath, "rev-list", "--count", "HEAD", "^@{u}")
	if err != nil {
		return false, err
	}

	return s != "0", nil
}

// HasUnpulledCommits checks if there are commits on the upstream
// branch that have not yet been pulled locally.
func HasUnpulledCommits(ctx context.Context, repoPath string) (bool, error) {
	// FIX: not implemented yet
	if err := HasUpstream(ctx, repoPath); err != nil {
		return false, err
	}

	// Count commits present in the upstream but not locally
	out, err := runWithOutput(ctx, repoPath, "rev-list", "--count", "@{u}", "^HEAD")
	if err != nil {
		return false, fmt.Errorf("checking unpulled commits: %w", err)
	}

	return strings.TrimSpace(out) != "0", nil
}

// HasUpstream checks whether the current branch has an upstream (remote tracking branch) configured.
func HasUpstream(ctx context.Context, repoPath string) error {
	err := runWithWriter(ctx, io.Discard, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return ErrGitNoUpstream
	}

	return nil
}

// commitChanges commits local changes.
func commitChanges(ctx context.Context, repoPath, msg string) error {
	return runGitCmd(ctx, repoPath, "commit", "-m", msg)
}

// hasChanges checks if there are any staged or unstaged changes in the repo.
func hasChanges(ctx context.Context, repoPath string) (bool, error) {
	output, err := runWithOutput(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return strings.TrimSpace(output) != "", nil
}

// initialize creates a new Git repository.
func initialize(ctx context.Context, repoPath string, force bool) error {
	if IsInitialized(repoPath) && !force {
		return ErrGitInitialized
	}

	if err := files.MkdirAll(repoPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	return runGitCmd(ctx, repoPath, "init")
}

// push pushes local changes to remote.
func push(ctx context.Context, repoPath string) error {
	// check if remote exists
	remotes, err := runWithOutput(ctx, repoPath, "remote")
	if err != nil {
		return fmt.Errorf("git remote check failed: %w", err)
	}

	if strings.TrimSpace(remotes) == "" {
		return ErrGitNoUpstream
	}

	branch, err := branch(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("could not get current branch: %w", err)
	}
	// check if branch has upstream
	err = runWithWriter(ctx, io.Discard, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		// no upstream, so set it
		return runGitCmd(ctx, repoPath, "push", "--set-upstream", "origin", branch)
	}

	return runGitCmd(ctx, repoPath, "push")
}

// status returns the status of the repo.
func status(ctx context.Context, repoPath string) (string, error) {
	if !hasCommits(ctx, repoPath) {
		return "", ErrGitNoCommits
	}

	added, modified, deleted, err := countStagedChanges(repoPath)
	if err != nil {
		return "", err
	}

	return formatStatus(added, modified, deleted), nil
}

func countStagedChanges(repoPath string) (added, modified, deleted int, err error) {
	var out bytes.Buffer
	cmd := exec.Command(gitCmd, "diff", "--cached", "--name-status")
	cmd.Stdout = &out
	cmd.Dir = repoPath

	if err := cmd.Run(); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to run git diff-tree: %w", err)
	}

	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
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

func formatStatus(added, modified, deleted int) string {
	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+add:%d", added))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("-del:%d", deleted))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("~mod:%d", modified))
	}
	return strings.Join(parts, " ")
}

// branch returns the current branch.
func branch(ctx context.Context, repoPath string) (string, error) {
	return runWithOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
}

// Remote returns the origin of the repository.
func Remote(ctx context.Context, repoPath string) (string, error) {
	return runWithOutput(ctx, repoPath, "config", "--get", "remote.origin.url")
}

func setConfigLocal(ctx context.Context, repoPath, key, value string) error {
	return runGitCmd(ctx, repoPath, "config", "--local", key, value)
}

// IsInitialized checks if the repo is initialized.
func IsInitialized(repoPath string) bool {
	return files.Exists(filepath.Join(repoPath, ".git"))
}

// hasCommits checks if the repo has commits.
func hasCommits(ctx context.Context, repoPath string) bool {
	err := runWithWriter(ctx, io.Discard, repoPath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return false
		}

		return false
	}

	return true
}

// runWithOutput executes a git command and returns the output.
func runWithOutput(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, gitCmd, args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

// runWithWriter executes a Git command and writes output to the provided io.Writer.
func runWithWriter(ctx context.Context, stdout io.Writer, repoPath string, s ...string) error {
	cmd := exec.CommandContext(ctx, gitCmd, s...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	o := strings.TrimSpace(string(output))

	if err != nil {
		//nolint:err113 //dynamic error is fine for command output
		return fmt.Errorf("%s", o)
	}

	if o != "" {
		_, _ = fmt.Fprintf(stdout, "%s\n", o)
	}

	return nil
}

// runGitCmd executes a Git command.
func runGitCmd(ctx context.Context, repoPath string, commands ...string) error {
	// FIX: inject `io.Writer`
	g, err := sys.Which(gitCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", err, g)
	}

	w := frame.New(frame.WithColorBorder(ansi.Yellow))
	defer w.Flush()

	commands = append([]string{g, "-C", repoPath}, commands...)
	w.Midln(ansi.Yellow.With(ansi.Italic).Sprint(strings.Join(commands, " "))).Flush()

	err = sys.ExecCmdWithWriter(ctx, w, nil, commands...)
	if err != nil {
		w.Error("")
		return err
	}

	return nil
}
