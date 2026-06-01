package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	ErrGitRepoEmpty       = errors.New("git: empty repository")
)

type CmdLogger func(commands []string)

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

// Commit commits local changes.
func Commit(ctx context.Context, repoPath, msg string) error {
	return runGitCmd(ctx, repoPath, "commit", "-m", msg)
}

// HasChanges checks if there are any staged or unstaged changes in the repo.
func HasChanges(ctx context.Context, repoPath string) (bool, error) {
	output, err := runWithOutput(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return strings.TrimSpace(output) != "", nil
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
	cmd := exec.Command(command, "diff", "--cached", "--name-status")
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

// IsInitialized checks if the repo is initialized.
func IsInitialized(repoPath string) bool {
	return fileExists(filepath.Join(repoPath, ".git"))
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
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

// runWithWriter executes a Git command and writes output to the provided io.Writer.
func runWithWriter(ctx context.Context, stdout io.Writer, repoPath string, s ...string) error {
	cmd := exec.CommandContext(ctx, command, s...)
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

func runGitCmd(ctx context.Context, repoPath string, commands ...string) error {
	g, err := New(repoPath)
	if err != nil {
		return err
	}
	cmd := []string{g.Bin()}
	if repoPath != "" {
		cmd = append(cmd, "-C", repoPath)
	}

	commands = append(cmd, commands...)

	return execCmdWithWriter(ctx, os.Stdout, nil, commands...)
}

// execCmdWithWriter runs a command with the given arguments and writes the
// output to the writer.
func execCmdWithWriter(ctx context.Context, w io.Writer, r io.Reader, s ...string) error {
	slog.Debug("ExecCmdWithWriter", "cmds", s)
	cmd := exec.CommandContext(ctx, s[0], s[1:]...)
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}
