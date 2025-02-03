package repo

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func TestRemoveUnusedTags(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	bs := testSliceBookmarks()
	ctx := context.Background()
	tt := r.Cfg.Tables

	// insert some records
	_ = r.execTx(ctx, func(tx *sqlx.Tx) error {
		err := r.insertBulk(ctx, tt.Main, tt.RecordsTags, bs)
		assert.NoError(t, err, "failed to insert records")
		return nil
	})

	toRemoveB, err := r.ByID(tt.Main, 1)
	assert.NoError(t, err, "failed to retrieve bookmark by ID")
	assert.NotNil(t, toRemoveB, "Bookmark was not deleted")
	// count tags
	tagsCount := CountRecords(r, tt.Tags)
	// delete record
	err = r.delete(ctx, tt.Main, toRemoveB)
	assert.NoError(t, err, "failed to delete record")
	// run RemoveUnusedTags
	err = removeUnusedTags(r)
	assert.NoError(t, err, "failed to remove unused tags")
	tagsCountAfterDel := CountRecords(r, tt.Tags)
	assert.NotEqual(t, tagsCount, tagsCountAfterDel, "tags count should be different")
}

func TestTagsCounter(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// Insert test data
	tags := []struct {
		id   int
		name string
	}{
		{1, "tag1"},
		{2, "tag2"},
		{3, "tag3"},
	}

	records := []struct {
		id     int
		url    string
		title  string
		desc   string
		tagIDs []int
	}{
		{1, "http://example1.com", "Example 1", "Description 1", []int{1, 2}},
		{2, "http://example2.com", "Example 2", "Description 2", []int{2}},
		{3, "http://example3.com", "Example 3", "Description 3", []int{}}, // No tags
	}

	for _, tag := range tags {
		_, err := r.DB.Exec("INSERT INTO tags (id, name) VALUES (?, ?)", tag.id, tag.name)
		assert.NoError(t, err, "failed to insert tag")
	}

	for _, record := range records {
		_, err := r.DB.Exec("INSERT INTO bookmarks (id, url, title, desc) VALUES (?, ?, ?, ?)",
			record.id, record.url, record.title, record.desc)
		assert.NoError(t, err, "failed to insert bookmark")

		for _, tagID := range record.tagIDs {
			_, err := r.DB.Exec("INSERT INTO bookmark_tags (bookmark_url, tag_id) VALUES (?, ?)",
				record.url, tagID)
			assert.NoError(t, err, "failed to insert bookmark tag association")
		}
	}
	// run tagscounter
	tagCounts, err := CounterTags(r)
	assert.NoError(t, err, "failed to count tags")
	// expected tag counts
	expectedCounts := map[string]int{
		"tag1": 1, // Used by one record
		"tag2": 2, // Used by two records
		"tag3": 0, // Not used
	}
	// verify results
	for tag, expectedCount := range expectedCounts {
		count, exists := tagCounts[tag]
		assert.True(t, exists, "tag '%s' should exist in the results", tag)
		assert.Equal(t, expectedCount, count)
	}
	// ensure no extra tags are in the result
	assert.Equal(t, len(expectedCounts), len(tagCounts))
}

func TestGetOrCreateTag(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)
	newTagName := "newtag"
	ttags := r.Cfg.Tables.Tags
	_ = r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		// test creating a new tag
		tagID, err := createTag(tx, ttags, newTagName)
		assert.NotEqual(t, int64(0), tagID, "CreateTag returned ID 0, expected a valid ID")
		assert.NoError(t, err, "failed to create tag")
		// verify tag was inserted
		newTagID, err := getTag(tx, ttags, newTagName)
		assert.NoError(t, err, "failed to get tag")
		assert.Equal(t, tagID, newTagID, "GetTag returned wrong ID for 'newtag' id: %d", newTagID)

		return nil
	})
}
