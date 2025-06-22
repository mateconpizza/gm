package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripSuffixes(t *testing.T) {
	t.Run("remove all suffixes", func(t *testing.T) {
		t.Parallel()

		want := "somefile"
		p := want + ".db.enc"
		got := StripSuffixes(p)
		assert.Equal(t, want, got)
	})

	t.Run("no suffixes", func(t *testing.T) {
		t.Parallel()

		want := "somefile"
		got := StripSuffixes(want)
		assert.Equal(t, want, got)
	})
}
