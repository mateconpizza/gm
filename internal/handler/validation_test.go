package handler

import (
	"testing"
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
