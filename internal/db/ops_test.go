//nolint:paralleltest //test
package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDropRepository(t *testing.T) {
	const n = 10
	r := testPopulatedDB(t, n)
	defer teardownthewall(r.DB)
	b, err := r.ByID(1)
	assert.NoError(t, err)
	assert.NotNil(t, b)
	err = Drop(r, t.Context())
	assert.NoError(t, err)
	b, err = r.ByID(1)
	assert.Nil(t, b)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ErrRecordNotFound.Error())
}

func TestCountRecords(t *testing.T) {
	const n = 12
	r := testPopulatedDB(t, n)
	defer teardownthewall(r.DB)
	count := countRecords(r, schemaMain.name)
	assert.Equal(t, n, count)
}
