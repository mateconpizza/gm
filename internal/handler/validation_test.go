package handler

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/locker"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/terminal"
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
		assert.ErrorIs(t, err, repo.ErrDBNotFound)
		assert.Empty(t, got)
	})
}

func TestPasswordInput(t *testing.T) {
	t.Run("valid password input", func(t *testing.T) {
		t.Parallel()
		pwd := "123"
		f := frame.New()
		input := strings.NewReader(pwd + "\n" + pwd + "\n")
		term := terminal.New(
			terminal.WithWriter(io.Discard),
			terminal.WithReader(input),
		)

		s, err := passwordConfirm(term, f)
		assert.NoError(t, err)
		assert.Equal(t, pwd, s)
	})
	t.Run("password mismatch", func(t *testing.T) {
		t.Parallel()
		f := frame.New()
		input := strings.NewReader("password1\npassword2\n")
		term := terminal.New(
			terminal.WithWriter(io.Discard),
			terminal.WithReader(input),
		)

		s, err := passwordConfirm(term, f)
		assert.Error(t, err)
		assert.Empty(t, s)
		assert.ErrorIs(t, err, locker.ErrPassphraseMismatch)
	})
}
