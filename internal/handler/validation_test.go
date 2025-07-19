package handler

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
)

func TestExtractIDsFromString(t *testing.T) {
	t.Parallel()

	t.Run("extract valid IDs", func(t *testing.T) {
		t.Parallel()
		idsStr := []string{"1", "2", "3"}
		ids, err := extractIDsFrom(idsStr)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		expected := []int{1, 2, 3}
		if !equalIntSlice(ids, expected) {
			t.Errorf("got %v, want %v", ids, expected)
		}
	})

	t.Run("invalid IDs", func(t *testing.T) {
		t.Parallel()
		nonIntStr := []string{"a", "b", "c"}
		ids, err := extractIDsFrom(nonIntStr)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected empty slice, got %v", ids)
		}
	})
}

func TestFindLockedDB(t *testing.T) {
	t.Run("success finding unlocked DB", func(t *testing.T) {
		t.Parallel()
		f := testSetupDBFiles(t, t.TempDir(), 12)
		want := f[0]
		got, err := FindDB(want)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("success finding DB without extension", func(t *testing.T) {
		t.Parallel()
		f := testSetupDBFiles(t, t.TempDir(), 10)
		want := f[0]
		withoutExt := strings.TrimSuffix(want, filepath.Ext(want))
		got, err := FindDB(withoutExt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("failed finding DB", func(t *testing.T) {
		t.Parallel()
		f := testSetupDBFiles(t, t.TempDir(), 10)
		path := filepath.Dir(f[0])
		nonExistentFile := filepath.Join(path, "non-existent.db")
		got, err := FindDB(nonExistentFile)
		if err == nil {
			t.Errorf("expected error, got none")
		}
		if !errors.Is(err, db.ErrDBNotFound) {
			t.Errorf("expected ErrDBNotFound, got %v", err)
		}
		if got != "" {
			t.Errorf("expected empty result, got %q", got)
		}
	})
}

func TestPasswordInput(t *testing.T) {
	t.Run("valid password input", func(t *testing.T) {
		t.Parallel()
		pwd := "123"
		input := strings.NewReader(pwd + "\n" + pwd + "\n")

		c := ui.NewConsole(
			ui.WithFrame(frame.New()),
			ui.WithTerminal(terminal.New(
				terminal.WithWriter(io.Discard),
				terminal.WithReader(input),
			)),
		)

		s, err := passwordConfirm(c)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if s != pwd {
			t.Errorf("got %q, want %q", s, pwd)
		}
	})

	t.Run("password mismatch", func(t *testing.T) {
		t.Parallel()
		input := strings.NewReader("password1\npassword2\n")
		c := ui.NewConsole(
			ui.WithFrame(frame.New()),
			ui.WithTerminal(terminal.New(
				terminal.WithWriter(io.Discard),
				terminal.WithReader(input),
			)),
		)

		s, err := passwordConfirm(c)
		if err == nil {
			t.Error("expected error, got none")
		}
		if !errors.Is(err, locker.ErrPassphraseMismatch) {
			t.Errorf("expected ErrPassphraseMismatch, got %v", err)
		}
		if s != "" {
			t.Errorf("expected empty string, got %q", s)
		}
	})
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
