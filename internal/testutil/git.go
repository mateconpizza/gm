package testutil

import (
	"context"
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/mateconpizza/gm/pkg/git"
)

type FakeResponse struct {
	out string
	err error
}

type FakeTokenResponse struct {
	token string
	resp  FakeResponse
}

type FakeGitExecuter struct {
	calls          [][]string
	out            string // fallback output for unconfigured subcommands
	err            error  // fallback error for unconfigured subcommands
	responses      map[string]FakeResponse
	tokenResponses []FakeTokenResponse
}

func (f *FakeGitExecuter) Output(ctx context.Context, dir string, args ...string) (string, error) {
	cmds := append([]string(nil), args...) // defensive copy - args' backing array can be reused by the caller
	f.calls = append(f.calls, cmds)

	if resp, ok := f.lookup(cmds); ok {
		return resp.out, resp.err
	}
	return f.out, f.err
}

func (f *FakeGitExecuter) Run(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
	cp := append([]string(nil), cmds...)
	f.calls = append(f.calls, cp)

	resp, ok := f.lookup(cp)
	if !ok {
		resp = FakeResponse{out: f.out, err: f.err}
	}
	if resp.out != "" {
		fmt.Fprint(w, resp.out)
	}
	return resp.err
}

// On configures a canned response for a subcommand.
func (f *FakeGitExecuter) On(subcommand, out string, err error) *FakeGitExecuter {
	if f.responses == nil {
		f.responses = make(map[string]FakeResponse)
	}
	f.responses[subcommand] = FakeResponse{out: out, err: err}
	return f
}

// OnContains configures a response for any call whose args contain token
// anywhere.
func (f *FakeGitExecuter) OnContains(token, out string, err error) *FakeGitExecuter {
	f.tokenResponses = append(f.tokenResponses, FakeTokenResponse{token: token, resp: FakeResponse{out: out, err: err}})
	return f
}

// lookup now checks subcommand-keyed responses first, then falls back to
// token matches in registration order, then f.out/f.err.
func (f *FakeGitExecuter) lookup(cmds []string) (FakeResponse, bool) {
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
	return FakeResponse{}, false
}

func NewGit(t *testing.T, path string, opts ...git.GitOpt) *git.Git {
	t.Helper()
	g, _ := git.New(path, opts...)
	return g
}
