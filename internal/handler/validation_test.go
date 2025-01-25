package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractIDsFromString(t *testing.T) {
	// valid ids
	idsStr := []string{
		"1", "2", "3",
	}
	ids, err := extractIDsFrom(idsStr)
	assert.NoError(t, err)
	assert.Equal(t, ids, []int{1, 2, 3})
	// invalid ids
	nonIntStr := []string{
		"a", "b", "c",
	}
	ids, err = extractIDsFrom(nonIntStr)
	assert.NoError(t, err)
	assert.Equal(t, ids, []int{})
}
