package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrGitInitialized     = errors.New("git: repo is initialized")
	ErrGitNotInitialized  = errors.New("git: repo is not initialized")
	ErrGitNoCommits       = errors.New("git: no commits found")
	ErrGitNoOrigin        = errors.New("git: no origin remote configured")
	ErrGitNoRemote        = errors.New("git: no remote configured")
	ErrGitNothingToCommit = errors.New("git: no changes to commit")
)

type SyncGitSummary struct {
	GitBranch          string `json:"git_branch"`
	GitRemote          string `json:"git_remote"`
	LastSync           string `json:"last_sync"`
	ConflictResolution string `json:"conflict_resolution"`
	HashAlgorithm      string `json:"hash_algorithm"`
	Version            string `json:"version"`
}

func NewSummary(repoPath, v string) *SyncGitSummary {
	return &SyncGitSummary{
		ConflictResolution: "timestamp",
		HashAlgorithm:      "checksum",
		Version:            v,
	}
}

func UpdatedSummary(repoPath, v string) (*SyncGitSummary, error) {
	branch, err := getBranch(repoPath)
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}
	remote, err := getRemote(repoPath)
	if err != nil {
		remote = ""
	}

	return &SyncGitSummary{
		GitBranch:          branch,
		GitRemote:          remote,
		LastSync:           time.Now().Format(time.RFC3339),
		ConflictResolution: "timestamp",
		HashAlgorithm:      "checksum",
		Version:            v,
	}, nil
}

// fileExists checks if a file fileExists.
func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

// AddAll adds all local changes.
func AddAll(repoPath string) error {
	return runWithWriter(os.Stdout, repoPath, "add", ".")
}

// AddRemote adds a remote repository.
func AddRemote(repoPath, reporURL string) error {
	slog.Debug("setting git remote", "path", repoPath)
	return runWithWriter(os.Stdout, repoPath, "remote", "add", "origin", reporURL)
}

// CommitChanges commits local changes.
func CommitChanges(repoPath, msg string) error {
	return runWithWriter(os.Stdout, repoPath, "commit", "-m", msg)
}

// Fetch pulls changes from remote repository.
func Fetch(repoPath string) error {
	// first, fetch to see if there are remote changes
	if err := runWithWriter(os.Stdout, repoPath, "fetch"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	branch, err := getBranch(repoPath)
	if err != nil {
		return fmt.Errorf("could not get current branch: %w", err)
	}
	// pull the changes <pull>
	return runWithWriter(os.Stdout, repoPath, "pull", "origin", branch)
}

// HasChanges checks if there are any staged or unstaged changes in the repo.
func HasChanges(repoPath string) (bool, error) {
	output, err := runWithOutput(repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

// InitRepo creates a new Git repository.
func InitRepo(repoPath string, force bool) error {
	slog.Debug("initializing git", "path", repoPath)
	if IsInitialized(repoPath) && !force {
		return ErrGitInitialized
	}
	return runWithWriter(os.Stdout, repoPath, "init")
}

// Log returns the log of the repo.
func Log(repoPath string) error {
	return runWithWriter(
		os.Stdout,
		repoPath,
		"log",
		"--pretty=format:'%h %ad | %s%d [%an]'",
		"--graph",
		"--date=short",
	)
}

// PushChanges pushes local changes to remote.
func PushChanges(repoPath string) error {
	slog.Debug("pushing git changes", "path", repoPath)
	// check if remote exists
	remotes, err := runWithOutput(repoPath, "remote")
	if err != nil {
		return fmt.Errorf("git remote check failed: %w", err)
	}
	if strings.TrimSpace(remotes) == "" {
		return ErrGitNoRemote
	}
	branch, err := getBranch(repoPath)
	if err != nil {
		return fmt.Errorf("could not get current branch: %w", err)
	}
	// check if branch has upstream
	err = runWithWriter(io.Discard, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		// no upstream, so set it
		slog.Debug("no upstream set, using --set-upstream", "branch", branch)
		return runWithWriter(os.Stdout, repoPath, "push", "--set-upstream", "origin", branch)
	}

	return runWithWriter(os.Stdout, repoPath, "push")
}

// Status returns the status of the repo.
func Status(repoPath string) (string, error) {
	if !hasCommits(repoPath) {
		return "", ErrGitNoCommits
	}

	var out bytes.Buffer
	cmd := exec.Command("git", "diff", "--cached", "--name-status")
	cmd.Stdout = &out
	cmd.Dir = repoPath

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run git diff-tree: %w", err)
	}

	var added, modified, deleted int
	lines := strings.SplitSeq(strings.TrimSpace(out.String()), "\n")
	for line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		switch status {
		case "A":
			added++
		case "M":
			modified++
		case "D":
			deleted++
		}
	}

	return fmt.Sprintf("Added: %d, Modified: %d, Deleted: %d", added, modified, deleted), nil
}

// getBranch returns the current branch.
func getBranch(repoPath string) (string, error) {
	return runWithOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
}

// getRemote returns the origin of the repository.
func getRemote(repoPath string) (string, error) {
	slog.Debug("getting git remote", "path", repoPath)
	return runWithOutput(repoPath, "config", "--get", "remote.origin.url")
}

// IsInitialized checks if the repo is initialized.
func IsInitialized(repoPath string) bool {
	slog.Debug("checking if git is initialized", "path", repoPath)
	return fileExists(filepath.Join(repoPath, ".git"))
}

// hasCommits checks if the repo has commits.
func hasCommits(repoPath string) bool {
	slog.Debug("checking if git has commits", "path", repoPath)
	err := runWithWriter(io.Discard, repoPath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			return false
		}
		return false
	}
	return true
}

// runWithOutput executes a git command and returns the output.
func runWithOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// runWithWriter executes a Git command and writes output to the provided io.Writer.
func runWithWriter(stdout io.Writer, repoPath string, s ...string) error {
	cmd := exec.Command("git", s...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	o := strings.TrimSpace(string(output))
	if err != nil {
		//nolint:err113 //notneeded
		return fmt.Errorf("%s", o)
	}
	if o != "" {
		_, _ = fmt.Fprintf(stdout, "%s\n", o)
	}
	return nil
}
