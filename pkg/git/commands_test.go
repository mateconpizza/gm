package git

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestCommander_Output(t *testing.T) {
	t.Parallel()

	fake := func(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
		fmt.Fprint(w, "3\n")
		return nil
	}

	c := NewCommander("git").WithExecutor(fake)
	out, err := c.Output(t.Context(), "/repo", "rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "3" {
		t.Errorf("got %q, want %q", out, "3")
	}
}

func TestHasUnpushedCommits(t *testing.T) {
	t.Parallel()

	fe := &fakeGitExecuter{out: "3\n"}
	g, err := New("/repo", WithExecuter(fe.run))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := g.HasUnpushedCommits(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("want true, got false")
	}
}
