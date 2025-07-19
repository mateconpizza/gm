//nolint:wsl //test
package handler

import (
	"os"
	"testing"
)

func testSetupDBFiles(t *testing.T, tempDir string, n int) []string {
	t.Helper()
	r := make([]string, 0, n)

	for range n {
		tf, err := os.CreateTemp(tempDir, "sqlite-*.db")
		if err != nil {
			t.Fatal(err)
		}

		r = append(r, tf.Name())
	}

	return r
}

func TestRemoveRepo(t *testing.T) {
	t.Skip("skipping for now")
	t.Parallel()
	fs := testSetupDBFiles(t, t.TempDir(), 10)
	_ = fs
}

func TestRemoveBackups(t *testing.T) {
	t.Parallel()
	t.Skip("skipping for now")
}
