package editor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type fakeTerminal struct {
	chooseAnswer string
	chooseErr    error
	chooseCalls  int
}

func (f *fakeTerminal) Choose(_ context.Context, _ string, _ []string, _ string) (string, error) {
	f.chooseCalls++
	return f.chooseAnswer, f.chooseErr
}

func (f *fakeTerminal) SuccessMesg(a ...any) string { return "ok" }

type fakeEditor struct {
	out []byte
	err error
}

func (f *fakeEditor) Edit(_ context.Context, _ []byte, _ string) ([]byte, error) {
	return f.out, f.err
}

type fakeStrategy struct {
	buf      []byte
	parsed   *bookmark.Bookmark
	parseErr error
}

func (f *fakeStrategy) BuildBuffer(_ *editor.Meta, _ *bookmark.Bookmark, _, _ int) ([]byte, error) {
	return f.buf, nil
}

func (f *fakeStrategy) ParseBuffer(_ context.Context, _ []byte, _ *bookmark.Bookmark) (*bookmark.Bookmark, error) {
	return f.parsed, f.parseErr
}

func (f *fakeStrategy) Diff(_, _ *bookmark.Bookmark) string { return "diff" }
func (f *fakeStrategy) FileType() string                    { return "bm" }

func TestEditSessionRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		chooseAnswer     string
		parsed           *bookmark.Bookmark
		parseErr         error
		wantPersistCalls int
		wantErr          bool
	}{
		{
			name:             "user_confirms_save",
			chooseAnswer:     "yes",
			parsed:           &bookmark.Bookmark{ID: 1, Title: "new title"},
			wantPersistCalls: 1,
		},
		{
			name:             "user_declines_save",
			chooseAnswer:     "no",
			parsed:           &bookmark.Bookmark{ID: 1, Title: "new title"},
			wantPersistCalls: 0,
		},
		{
			name:     "buffer_unchanged_skips_prompt",
			parsed:   nil,
			parseErr: editor.ErrBufferUnchanged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			strategy := &fakeStrategy{
				buf:      []byte("original"),
				parsed:   tt.parsed,
				parseErr: tt.parseErr,
			}
			term := &fakeTerminal{chooseAnswer: tt.chooseAnswer}

			var persistCalls int
			session := editor.NewEditSession().
				WithStrategy(strategy).
				WithTerminal(term).
				WithEditor(&fakeEditor{out: []byte("edited")}).
				WithPersistFunc(func(_ context.Context, _, _ *bookmark.Bookmark) error {
					persistCalls++
					return nil
				})

			err := session.Run(t.Context(), []*bookmark.Bookmark{{ID: 1}})

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Run() expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() unexpected error: %v", err)
			}
			if persistCalls != tt.wantPersistCalls {
				t.Fatalf("persist called %d times; want %d", persistCalls, tt.wantPersistCalls)
			}
		})
	}
}

func TestEditSessionValidate(t *testing.T) {
	tests := []struct {
		name         string
		withStrategy bool
		withTerm     bool
		withEditor   bool
		withPersist  bool
		wantErrMsg   string // substring expected in the error, empty means no error
	}{
		{
			name:         "all_deps_set_no_error",
			withStrategy: true,
			withTerm:     true,
			withEditor:   true,
			withPersist:  true,
		},
		{
			name:         "missing_strategy_checked_first",
			withStrategy: false,
			withTerm:     true,
			withEditor:   true,
			withPersist:  true,
			wantErrMsg:   "WithStrategy",
		},
		{
			name:         "missing_terminal",
			withStrategy: true,
			withTerm:     false,
			withEditor:   true,
			withPersist:  true,
			wantErrMsg:   "WithTerminal",
		},
		{
			name:         "missing_editor",
			withStrategy: true,
			withTerm:     true,
			withEditor:   false,
			withPersist:  true,
			wantErrMsg:   "WithEditor",
		},
		{
			name:         "missing_persist_func",
			withStrategy: true,
			withTerm:     true,
			withEditor:   true,
			withPersist:  false,
			wantErrMsg:   "WithPersistFunc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := editor.NewEditSession()

			if tt.withStrategy {
				session = session.WithStrategy(&fakeStrategy{})
			}
			if tt.withTerm {
				session = session.WithTerminal(&fakeTerminal{})
			}
			if tt.withEditor {
				session = session.WithEditor(&fakeEditor{})
			}
			if tt.withPersist {
				session = session.WithPersistFunc(func(context.Context, *bookmark.Bookmark, *bookmark.Bookmark) error {
					return nil
				})
			}

			err := session.Run(t.Context(), []*bookmark.Bookmark{})
			if tt.wantErrMsg == "" {
				if err != nil {
					t.Fatalf("validate() unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validate() expected an error containing %q, got nil", tt.wantErrMsg)
			}
			if !errors.Is(err, editor.ErrMissingDep) {
				t.Fatalf("validate() error = %v; want wrapped ErrMissingDep", err)
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Fatalf("validate() error = %q; want substring %q", err.Error(), tt.wantErrMsg)
			}
		})
	}
}
