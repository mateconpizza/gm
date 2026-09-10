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
	ErrGitInitialized    = errors.New("git: is initialized")
	ErrGitNotInitialized = errors.New("git: is not initialized")
	ErrGitDisabled       = errors.New("git: is disabled")
	ErrGitNoCommits      = errors.New("git: no commits found")
	ErrGitNoUpstream     = errors.New("git: no upstream configured")
	ErrGitUpstreamExists = errors.New("git: remote origin already exists")
	ErrGitUpToDate       = errors.New("git: everything up-to-date")
	ErrGitRepoEmpty      = errors.New("git: empty repository")
)

// Commander runs a single named binary, routing every invocation through
// an injectable ExecuterFunc.
type Commander struct {
	bin      string
	executer ExecuterFunc
}

func NewCommander(bin string) *Commander {
	return &Commander{bin: bin, executer: defaultExecuter}
}

func (c *Commander) WithExecutor(fn ExecuterFunc) *Commander {
	c.executer = fn
	return c
}

// Output runs the command in dir and returns trimmed combined output.
func (c *Commander) Output(ctx context.Context, dir string, args ...string) (string, error) {
	var buf bytes.Buffer
	err := c.executer(ctx, dir, &buf, nil, append([]string{c.bin}, args...)...)
	return strings.TrimSpace(buf.String()), err
}

// Run executes the command in dir and streams trimmed output to w. A
// failing exit is turned into an error carrying that output, matching the
// original runWithWriter behavior.
func (c *Commander) Run(ctx context.Context, w io.Writer, dir string, args ...string) error {
	var buf bytes.Buffer
	err := c.executer(ctx, dir, &buf, nil, append([]string{c.bin}, args...)...)
	o := strings.TrimSpace(buf.String())
	if err != nil {
		//nolint:err113 //dynamic error is fine for command output
		return fmt.Errorf("%s", o)
	}
	if o != "" {
		fmt.Fprintf(w, "%s\n", o)
	}
	return nil
}

func (c *Commander) Exec(ctx context.Context, w io.Writer, dir string, args ...string) error {
	return c.executer(ctx, dir, w, nil, append([]string{c.bin}, args...)...)
}

// Remote returns the origin of the repository.
func Remote(ctx context.Context, repoPath string) (string, error) {
	return NewCommander(command).Output(ctx, repoPath, "config", "--get", "remote.origin.url")
}

func Run(ctx context.Context, repoPath string, commands ...string) error {
	return NewCommander(command).Run(ctx, os.Stdout, repoPath, commands...)
}

// defaultExecuter runs a command with the given arguments and writes the
// output to the writer.
func defaultExecuter(ctx context.Context, dir string, w io.Writer, r io.Reader, s ...string) error {
	slog.Debug("ExecCmdWithWriter", "cmds", s)
	cmd := exec.CommandContext(ctx, s[0], s[1:]...)
	cmd.Dir = dir
	cmd.Stdin = r
	cmd.Stdout = w
	cmd.Stderr = w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
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

// IsInitialized checks if the repo is initialized.
func IsInitialized(repoPath string) bool { return fileExists(filepath.Join(repoPath, ".git")) }
