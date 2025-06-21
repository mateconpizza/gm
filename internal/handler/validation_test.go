package handler

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

func TestExtractIDsFromString(t *testing.T) {
	t.Run("extract valid IDs", func(t *testing.T) {
		t.Parallel()
		idsStr := []string{"1", "2", "3"}
		ids, err := extractIDsFrom(idsStr)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, ids)
	})
	t.Run("invalid IDs", func(t *testing.T) {
		t.Parallel()
		nonIntStr := []string{"a", "b", "c"}
		ids, err := extractIDsFrom(nonIntStr)
		assert.NoError(t, err)
		assert.Equal(t, []int{}, ids)
	})
}

func TestFindLockedDB(t *testing.T) {
	t.Run("success finding unlocked DB", func(t *testing.T) {
		t.Parallel()
		f := testSetupDBFiles(t, t.TempDir(), 12)
		want := f[0]
		got, err := FindDB(want)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("success finding DB without extension", func(t *testing.T) {
		t.Parallel()
		f := testSetupDBFiles(t, t.TempDir(), 10)
		want := f[0]
		withoutExt := strings.TrimSuffix(want, filepath.Ext(want))
		got, err := FindDB(withoutExt)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})
	t.Run("failed finding DB", func(t *testing.T) {
		t.Parallel()
		f := testSetupDBFiles(t, t.TempDir(), 10)
		path := filepath.Dir(f[0])
		nonExistentFile := filepath.Join(path, "non-existent.db")
		got, err := FindDB(nonExistentFile)
		assert.Error(t, err)
		assert.ErrorIs(t, err, db.ErrDBNotFound)
		assert.Empty(t, got)
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
		assert.NoError(t, err)
		assert.Equal(t, pwd, s)
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
		assert.Error(t, err)
		assert.Empty(t, s)
		assert.ErrorIs(t, err, locker.ErrPassphraseMismatch)
	})
}
