package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

var (
	ErrGitInitialized     = errors.New("git: is initialized")
	ErrGitNotInitialized  = errors.New("git: is not initialized")
	ErrGitNoCommits       = errors.New("git: no commits found")
	ErrGitNoRemote        = errors.New("git: no upstream configured")
	ErrGitNothingToCommit = errors.New("git: no changes to commit")
	ErrGitRepoNotFound    = errors.New("git: repo not found")
	ErrGitRepoURLEmpty    = errors.New("git: repo url is empty")
)

const gitCmd = "git"

func Clone(destRepoPath, repoURL string) error {
	return RunGitCmd("", "clone", repoURL, destRepoPath)
}

// AddAll adds all local changes.
func AddAll(repoPath string) error {
	return RunGitCmd(repoPath, "add", ".")
}

// addRemote adds a remote repository.
func addRemote(repoPath, repoURL string) error {
	if config.App.Force {
		return RunGitCmd(repoPath, "remote", "set-url", "origin", repoURL)
	}

	return RunGitCmd(repoPath, "remote", "add", "origin", repoURL)
}

func SjtUpstream(repoPath string) error {
	b, err := GetBranch(repoPath)
	if err != nil {
		return err
	}
	return RunGitCmd(repoPath, "push", "--set-upstream", "origin", b)
}

// HasUnpushedCommits checks if there are any unpushed commits.
func HasUnpushedCommits(repoPath string) (bool, error) {
	err := RunGitCmd(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		return false, err
	}

	s, err := runWithOutput(repoPath, "rev-list", "--count", "HEAD", "^@{u}")
	if err != nil {
		return false, err
	}

	return s != "0", nil
}

// CommitChanges commits local changes.
func CommitChanges(repoPath, msg string) error {
	return RunGitCmd(repoPath, "commit", "-m", msg)
}

// HasChanges checks if there are any staged or unstaged changes in the repo.
func HasChanges(repoPath string) (bool, error) {
	output, err := runWithOutput(repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

// initialize creates a new Git repository.
func initialize(repoPath string, force bool) error {
	if IsInitialized(repoPath) && !force {
		return ErrGitInitialized
	}

	if err := files.MkdirAll(repoPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	return RunGitCmd(repoPath, "init")
}

// push pushes local changes to remote.
func push(repoPath string) error {
	// check if remote exists
	remotes, err := runWithOutput(repoPath, "remote")
	if err != nil {
		return fmt.Errorf("git remote check failed: %w", err)
	}
	if strings.TrimSpace(remotes) == "" {
		return ErrGitNoRemote
	}
	branch, err := GetBranch(repoPath)
	if err != nil {
		return fmt.Errorf("could not get current branch: %w", err)
	}
	// check if branch has upstream
	err = runWithWriter(io.Discard, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		// no upstream, so set it
		return RunGitCmd(repoPath, "push", "--set-upstream", "origin", branch)
	}

	return RunGitCmd(repoPath, "push")
}

// Status returns the status of the repo.
func Status(repoPath string) (string, error) {
	if !hasCommits(repoPath) {
		return "", ErrGitNoCommits
	}

	var out bytes.Buffer
	cmd := exec.Command(gitCmd, "diff", "--cached", "--name-status")
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

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("Add:%d", added))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("Del:%d", deleted))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("Mod:%d", modified))
	}

	return strings.TrimSpace(strings.Join(parts, " ")), nil
}

// GetBranch returns the current branch.
func GetBranch(repoPath string) (string, error) {
	return runWithOutput(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
}

// GetRemote returns the origin of the repository.
func GetRemote(repoPath string) (string, error) {
	return runWithOutput(repoPath, "config", "--get", "remote.origin.url")
}

// IsInitialized checks if the repo is initialized.
func IsInitialized(repoPath string) bool {
	return files.Exists(filepath.Join(repoPath, ".git"))
}

// hasCommits checks if the repo has commits.
func hasCommits(repoPath string) bool {
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
	cmd := exec.Command(gitCmd, args...)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// runWithWriter executes a Git command and writes output to the provided io.Writer.
func runWithWriter(stdout io.Writer, repoPath string, s ...string) error {
	cmd := exec.Command(gitCmd, s...)
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

// RunGitCmd executes a Git command.
func RunGitCmd(repoPath string, commands ...string) error {
	gitCommand, err := sys.Which(gitCmd)
	if err != nil {
		return fmt.Errorf("%w: %s", err, gitCommand)
	}

	f := frame.New(frame.WithColorBorder(color.BrightOrange))
	defer f.Flush()

	commands = append([]string{gitCommand, "-C", repoPath}, commands...)
	cmdColors := color.ApplyMany(slices.Clone(commands), color.Gray, color.StyleItalic)
	f.Midln(strings.Join(cmdColors, " ")).Flush()

	err = sys.ExecCmdWithWriter(f, commands...)
	if err != nil {
		f.Error("")
		return fmt.Errorf("%w", err)
	}

	return nil
}
