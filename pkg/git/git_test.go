package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type response struct {
	out string
	err error
}

type tokenResponse struct {
	token string
	resp  response
}

type fakeGitExecuter struct {
	calls          [][]string
	out            string // fallback output for unconfigured subcommands
	err            error  // fallback error for unconfigured subcommands
	responses      map[string]response
	tokenResponses []tokenResponse
}

func (f *fakeGitExecuter) Output(ctx context.Context, dir string, args ...string) (string, error) {
	cmds := append([]string(nil), args...) // defensive copy - args' backing array can be reused by the caller
	f.calls = append(f.calls, cmds)

	if resp, ok := f.lookup(cmds); ok {
		return resp.out, resp.err
	}
	return f.out, f.err
}

func (f *fakeGitExecuter) run(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
	cp := append([]string(nil), cmds...)
	f.calls = append(f.calls, cp)

	resp, ok := f.lookup(cp)
	if !ok {
		resp = response{out: f.out, err: f.err}
	}
	if resp.out != "" {
		fmt.Fprint(w, resp.out)
	}
	return resp.err
}

// lookup now checks subcommand-keyed responses first, then falls back to
// token matches in registration order, then f.out/f.err.
func (f *fakeGitExecuter) lookup(cmds []string) (response, bool) {
	if len(cmds) >= 2 {
		if resp, ok := f.responses[cmds[1]]; ok {
			return resp, true
		}
	}
	for _, tr := range f.tokenResponses {
		if slices.Contains(cmds, tr.token) {
			return tr.resp, true
		}
	}
	return response{}, false
}

// on configures a canned response for a subcommand.
func (f *fakeGitExecuter) on(subcommand, out string, err error) *fakeGitExecuter {
	if f.responses == nil {
		f.responses = make(map[string]response)
	}
	f.responses[subcommand] = response{out: out, err: err}
	return f
}

// onContains configures a response for any call whose args contain token
// anywhere.
func (f *fakeGitExecuter) onContains(token, out string, err error) *fakeGitExecuter {
	f.tokenResponses = append(f.tokenResponses, tokenResponse{token: token, resp: response{out: out, err: err}})
	return f
}

func TestGit_Commit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msg     string
		fake    *fakeGitExecuter
		wantErr bool
	}{
		{
			name: "success",
			msg:  "fix: update readme",
			fake: &fakeGitExecuter{},
		},
		{
			name:    "nothing to commit",
			msg:     "fix: update readme",
			fake:    &fakeGitExecuter{out: "nothing to commit, working tree clean", err: ErrGit},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g, err := New("/repo", WithExecuter(tt.fake.run))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			err = g.Commit(t.Context(), tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Commit() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotArgs := tt.fake.calls[0]
			wantArgs := []string{"commit", "-m", tt.msg}
			if !reflect.DeepEqual(lastN(gotArgs, len(wantArgs)), wantArgs) {
				t.Errorf("args = %v, want suffix %v", gotArgs, wantArgs)
			}
		})
	}
}

func TestGit_SetCfgLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		k       string
		v       string
		fake    *fakeGitExecuter
		wantErr bool
	}{
		{
			name: "typical_config",
			k:    "user.name",
			v:    "Test User",
			fake: &fakeGitExecuter{},
		},
		{
			name: "empty_value",
			k:    "core.editor",
			v:    "",
			fake: &fakeGitExecuter{},
		},
		{
			name: "empty_key",
			k:    "",
			v:    "some_value",
			fake: &fakeGitExecuter{},
		},
		{
			name: "empty_key_and_value",
			k:    "",
			v:    "",
			fake: &fakeGitExecuter{},
		},
		{
			name: "boolean_string_value",
			k:    "core.filemode",
			v:    "false",
			fake: &fakeGitExecuter{},
		},
		{
			name: "special_characters_in_value",
			k:    "remote.origin.url",
			v:    "https://user:pass@github.com/repo.git",
			fake: &fakeGitExecuter{},
		},
		{
			name:    "git_command_failure",
			k:       "invalid.key",
			v:       "value",
			fake:    &fakeGitExecuter{err: errors.New("exit status 1")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g, err := New("/repo", WithExecuter(tt.fake.run))
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			err = g.SetCfgLocal(t.Context(), tt.k, tt.v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetCfgLocal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if len(tt.fake.calls) == 0 {
					t.Fatal("expected executer to be called")
				}

				gotArgs := tt.fake.calls[0]
				wantArgs := []string{"config", "--local", tt.k, tt.v}

				if !reflect.DeepEqual(lastN(gotArgs, len(wantArgs)), wantArgs) {
					t.Errorf("args = %v, want suffix %v", gotArgs, wantArgs)
				}
			}
		})
	}
}

func TestGit_HasUnpushedCommits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cmdOut  string
		cmdErr  error
		want    bool
		wantErr bool
	}{
		{
			name:    "no_unpushed_commits",
			cmdOut:  "0",
			cmdErr:  nil,
			want:    false,
			wantErr: false,
		},
		{
			name:    "has_unpushed_commits",
			cmdOut:  "5",
			cmdErr:  nil,
			want:    true,
			wantErr: false,
		},
		{
			name:    "negative_commits_count",
			cmdOut:  "-1",
			cmdErr:  nil,
			want:    true,
			wantErr: false,
		},
		{
			name:    "empty_output_parsing_error",
			cmdOut:  "",
			cmdErr:  nil,
			want:    false,
			wantErr: true,
		},
		{
			name:    "non_integer_output",
			cmdOut:  "abc",
			cmdErr:  nil,
			want:    false,
			wantErr: true,
		},
		{
			name:    "git_command_failure",
			cmdOut:  "",
			cmdErr:  ErrGit,
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeGitExecuter{
				out: tt.cmdOut,
				err: tt.cmdErr,
			}

			// Construct Git directly since it relies on the cmd interface
			g, err := New("/repo", WithExecuter(fake.run))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := g.HasUnpushedCommits(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("HasUnpushedCommits() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("HasUnpushedCommits() = %v, want %v", got, tt.want)
			}

			// Validate that the correct command was executed
			if len(fake.calls) > 0 {
				wantArgs := []string{command, "rev-list", "--count", "HEAD", "^@{u}"}
				if !reflect.DeepEqual(fake.calls[0], wantArgs) {
					t.Errorf("cmd.Output() args = %v, want %v", fake.calls[0], wantArgs)
				}
			}
		})
	}
}

func TestGit_Status(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		revParseErr error
		diffOut     string
		diffErr     error
		want        string
		wantErr     error
		wantErrMsg  string
	}{
		{
			name:    "typical_mixed_changes",
			diffOut: "A\tfile1.txt\nM\tfile2.txt\nD\tfile3.txt",
			want:    "+add:1 -del:1 ~mod:1",
		},
		{
			name:    "no_staged_changes",
			diffOut: "",
			want:    "",
		},
		{
			name:    "single_added_file",
			diffOut: "A\tfile.txt",
			want:    "+add:1",
		},
		{
			name:    "single_modified_file",
			diffOut: "M\tfile.txt",
			want:    "~mod:1",
		},
		{
			name:    "single_deleted_file",
			diffOut: "D\tfile.txt",
			want:    "-del:1",
		},
		{
			name:        "no_commits",
			revParseErr: ErrGit,
			wantErr:     ErrGitNoCommits,
		},
		{
			name:       "staged_changes_command_failure",
			diffErr:    errors.New("git diff failed"),
			wantErrMsg: "git diff failed",
		},
		{
			name:    "summary_file_only",
			diffOut: "A\t" + SummaryFileName,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls [][]string

			executer := func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
				t.Helper()

				calls = append(calls, append([]string(nil), cmds...))

				switch {
				case slices.Contains(cmds, "rev-parse"):
					return tt.revParseErr

				case slices.Contains(cmds, "diff"):
					if tt.diffErr != nil {
						return tt.diffErr
					}
					_, err := io.WriteString(w, tt.diffOut)
					return err

				default:
					t.Fatalf("unexpected command: %v", cmds)
					return nil
				}
			}

			g, err := New("/repo", WithExecuter(executer))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			got, err := g.Status(t.Context())

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Status() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("Status() error = nil, want error containing %q", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Status() error = %q, want error containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Status() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Status() = %q, want %q", got, tt.want)
			}

			if len(calls) != 2 {
				t.Fatalf("executer called %d times, want 2; calls = %v", len(calls), calls)
			}

			wantRevParse := []string{
				command,
				"rev-parse",
				"--verify",
				"HEAD",
			}
			if !slices.Equal(calls[0], wantRevParse) {
				t.Errorf("first command = %v, want %v", calls[0], wantRevParse)
			}

			wantDiff := []string{
				command,
				"diff",
				"--cached",
				"--name-status",
			}
			if !slices.Equal(calls[1], wantDiff) {
				t.Errorf("second command = %v, want %v", calls[1], wantDiff)
			}
		})
	}
}

func TestGit_HasUnpulledCommits(t *testing.T) {
	t.Parallel()

	errCheckingCommits := errors.New("rev-list failed")

	tests := []struct {
		name       string
		upstreamOK bool
		out        string
		cmdErr     error
		want       bool
		wantErr    error
		wantErrMsg string
	}{
		{
			name:       "upstream_has_commits",
			upstreamOK: true,
			out:        "3\n",
			want:       true,
		},
		{
			name:       "upstream_has_single_commit",
			upstreamOK: true,
			out:        "1",
			want:       true,
		},
		{
			name:       "upstream_is_up_to_date",
			upstreamOK: true,
			out:        "0\n",
			want:       false,
		},
		{
			name:       "empty_count",
			upstreamOK: true,
			out:        "",
			want:       true,
		},
		{
			name:       "whitespace_count",
			upstreamOK: true,
			out:        " \n\t0\t\n ",
			want:       false,
		},
		{
			name:       "checking_upstream_fails",
			upstreamOK: false,
			wantErr:    ErrGitNoUpstream,
		},
		{
			name:       "checking_unpulled_commits_fails",
			upstreamOK: true,
			cmdErr:     errCheckingCommits,
			wantErrMsg: "checking unpulled commits",
		},
		{
			name:       "large_commit_count",
			upstreamOK: true,
			out:        "999999999\n",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			executer := func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
				t.Helper()

				switch {
				case slices.Contains(cmds, "rev-parse"):
					if !tt.upstreamOK {
						return errors.New("no upstream")
					}
					return nil

				case slices.Contains(cmds, "rev-list"):
					if tt.cmdErr != nil {
						return tt.cmdErr
					}
					_, err := io.WriteString(w, tt.out)
					return err

				default:
					t.Fatalf("unexpected command: %v", cmds)
					return nil
				}
			}

			g, err := New("/repo", WithExecuter(executer))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			got, err := g.HasUnpulledCommits(t.Context())

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("HasUnpulledCommits() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("HasUnpulledCommits() error = nil, want error containing %q", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("HasUnpulledCommits() error = %q, want error containing %q", err, tt.wantErrMsg)
				}
				if !errors.Is(err, errCheckingCommits) {
					t.Fatalf("HasUnpulledCommits() error = %v, want wrapped %v", err, errCheckingCommits)
				}
				return
			}

			if err != nil {
				t.Fatalf("HasUnpulledCommits() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("HasUnpulledCommits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGit_HasChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		output     string
		cmdErr     error
		want       bool
		wantErrMsg string
	}{
		{
			name:   "has_changes",
			output: " M file.txt\n",
			want:   true,
		},
		{
			name:   "no_changes",
			output: "",
			want:   false,
		},
		{
			name:   "whitespace_only",
			output: " \n\t\n",
			want:   false,
		},
		{
			name:       "command_failure",
			cmdErr:     ErrGit,
			wantErrMsg: "git status failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			executer := func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
				t.Helper()

				wantArgs := []string{
					command,
					"status",
					"--porcelain",
				}
				if !slices.Equal(cmds, wantArgs) {
					t.Errorf("command = %v, want %v", cmds, wantArgs)
				}

				if tt.cmdErr != nil {
					return tt.cmdErr
				}

				_, err := io.WriteString(w, tt.output)
				return err
			}

			g, err := New("/repo", WithExecuter(executer))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			got, err := g.HasChanges(t.Context())

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("HasChanges() error = nil, want error containing %q", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("HasChanges() error = %q, want error containing %q", err, tt.wantErrMsg)
				}
				if !errors.Is(err, ErrGit) {
					t.Fatalf("HasChanges() error = %v, want wrapped %v", err, ErrGit)
				}
				return
			}

			if err != nil {
				t.Fatalf("HasChanges() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("HasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGit_Push(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		remoteOut   string
		remoteErr   error
		branchOut   string
		branchErr   error
		upstreamErr error
		pushErr     error
		wantErr     error
		wantErrMsg  string
	}{
		{
			name:        "normal_push_with_upstream",
			remoteOut:   "origin\n",
			branchOut:   "main\n",
			upstreamErr: nil,
			pushErr:     nil,
			wantErr:     nil,
		},
		{
			name:        "normal_push_set_upstream",
			remoteOut:   "origin\n",
			branchOut:   "main\n",
			upstreamErr: errors.New("no upstream"),
			pushErr:     nil,
			wantErr:     nil,
		},
		{
			name:       "remote_check_failure",
			remoteErr:  ErrGit,
			wantErrMsg: "git remote check failed",
		},
		{
			name:      "no_remotes_empty",
			remoteOut: "",
			wantErr:   ErrGitNoUpstream,
		},
		{
			name:       "branch_check_failure",
			remoteOut:  "origin\n",
			branchErr:  ErrGit,
			wantErrMsg: "could not get current branch",
		},
		{
			name:        "push_failure",
			remoteOut:   "origin\n",
			branchOut:   "main\n",
			upstreamErr: nil,
			pushErr:     ErrGit,
			wantErrMsg:  ErrGit.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callCount := 0
			executer := func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
				t.Helper()
				callCount++

				switch callCount {
				case 1:
					// remote check
					if tt.remoteErr != nil {
						return tt.remoteErr
					}
					_, err := io.WriteString(w, tt.remoteOut)
					return err
				case 2:
					// Branch() calls rev-parse --abbrev-ref HEAD
					if tt.branchErr != nil {
						return tt.branchErr
					}
					_, err := io.WriteString(w, tt.branchOut)
					return err
				case 3:
					// upstream check (rev-parse --abbrev-ref --symbolic-full-name @{u})
					return tt.upstreamErr
				case 4:
					// push or push --set-upstream
					return tt.pushErr
				}

				return nil
			}

			g, err := New("/repo", WithExecuter(executer))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			err = g.Push(t.Context())

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Push() error = nil, want error %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Push() error = %v, want error %v", err, tt.wantErr)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("Push() error = nil, want error containing %q", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Push() error = %q, want error containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Push() unexpected error: %v", err)
			}
		})
	}
}

func TestGit_Init(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		force      bool
		setupRepo  bool
		runErr     error
		wantErr    error
		wantErrMsg string
	}{
		{
			name:      "success_new_repo",
			force:     false,
			setupRepo: false,
			wantErr:   nil,
		},
		{
			name:      "already_initialized_no_force",
			force:     false,
			setupRepo: true,
			wantErr:   ErrGitInitialized,
		},
		{
			name:      "already_initialized_with_force",
			force:     true,
			setupRepo: true,
			wantErr:   nil,
		},
		{
			name:       "git_init_command_failure",
			force:      false,
			setupRepo:  false,
			runErr:     errors.New("git init failed"),
			wantErrMsg: "git init failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			repoPath := filepath.Join(tmpDir, "repo")

			if tt.setupRepo {
				if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
					t.Fatalf("failed to setup repo: %v", err)
				}
			}

			executer := func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
				if tt.runErr != nil {
					return tt.runErr
				}
				return os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
			}

			g, err := New(repoPath, WithExecuter(executer))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			err = g.Init(t.Context(), tt.force)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Init() error = nil, want error %v", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Init() error = %v, want error %v", err, tt.wantErr)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("Init() error = nil, want error containing %q", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Init() error = %q, want error containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Init() unexpected error: %v", err)
			}
		})
	}
}

func TestGit_unpushedCommitsCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cmdOutput  string
		cmdErr     error
		want       int
		wantErr    bool
		wantErrMsg error
	}{
		{
			name:      "normal_count",
			cmdOutput: "5",
			cmdErr:    nil,
			want:      5,
			wantErr:   false,
		},
		{
			name:      "zero_count",
			cmdOutput: "0",
			cmdErr:    nil,
			want:      0,
			wantErr:   false,
		},
		{
			name:      "large_count",
			cmdOutput: "2147483647", // MaxInt32
			cmdErr:    nil,
			want:      2147483647,
			wantErr:   false,
		},
		{
			name:       "cmd_error",
			cmdOutput:  "",
			cmdErr:     ErrGit,
			want:       0,
			wantErr:    true,
			wantErrMsg: ErrGit,
		},
		{
			name:       "empty_string_parse_error",
			cmdOutput:  "",
			cmdErr:     nil,
			want:       0,
			wantErr:    true,
			wantErrMsg: strconv.ErrSyntax,
		},
		{
			name:       "invalid_string_parse_error",
			cmdOutput:  "abc",
			cmdErr:     nil,
			want:       0,
			wantErr:    true,
			wantErrMsg: strconv.ErrSyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := fakeGitExecuter{
				out: tt.cmdOutput,
				err: tt.cmdErr,
			}

			g, err := New("/repo", WithExecuter(fake.run))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			got, err := g.unpushedCommitsCount(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatalf("unpushedCommitsCount() expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !errors.Is(err, tt.wantErrMsg) {
					t.Fatalf("unpushedCommitsCount() error = %v, wantErrMsg to contain %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unpushedCommitsCount() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("unpushedCommitsCount() = %d; want %d", got, tt.want)
			}
		})
	}
}

func lastN(s []string, n int) []string {
	if len(s) < n {
		return s
	}
	return s[len(s)-n:]
}
