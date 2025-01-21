package repo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveUnusedTags(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// insert test tags
	_, err := r.DB.Exec(`INSERT INTO tags (id, name) VALUES (1, 'tag1'), (2, 'tag2'), (3, 'tag3')`)
	assert.NoError(t, err, "failed to insert tags")
	// insert test data
	_, err = r.DB.Exec(
		`INSERT INTO bookmarks (id, url, title, desc) VALUES (1, 'http://example.com', 'Example', 'Description')`,
	)
	assert.NoError(t, err, "failed to insert test data")
	// insert association
	_, err = r.DB.Exec(`INSERT INTO bookmark_tags (bookmark_url, tag_id) VALUES (1, 1), (1, 2)`)
	assert.NoError(t, err, "failed to insert association")

	assert.NoError(t, r.RemoveUnusedTags(), "failed to remove unused tags")

	rows, err := r.DB.Query(`SELECT id, name FROM tags ORDER BY id`)
	assert.NoError(t, err, "failed to query tags table")
	defer func() {
		assert.NoError(t, rows.Close(), "failed to close rows")
	}()
	assert.NoError(t, rows.Err(), "failed to iterate rows")

	var remainingTags []string
	for rows.Next() {
		var id int
		var name string
		assert.NoError(t, rows.Scan(&id, &name), "failed to iterate rows")
		remainingTags = append(remainingTags, name)
	}

	expectedTags := []string{"tag1", "tag2"}
	assert.Equal(t, expectedTags, remainingTags)
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

func TestGetTag(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tx, err := r.DB.Beginx()
	assert.NoError(t, err, "failed to start transaction")

	// setup test data
	ttags := r.Cfg.Tables.Tags
	_, err = tx.Exec(
		fmt.Sprintf("INSERT INTO %s (id, name) VALUES (1, 'tag1'), (2, 'tag2')", ttags),
	)
	assert.NoError(t, err, "failed to insert test data")

	// existing tag
	tagID, err := r.getTag(tx, ttags, "tag1")
	assert.NoError(t, err, "failed to get tag")
	assert.Equal(t, tagID, int64(1), "GetTag returned wrong ID for 'tag1' id: %d", tagID)

	// non-existent tag
	tagID, err = r.getTag(tx, ttags, "nonexistent")
	assert.NoError(t, err, "failed to get non-existent tag")
	assert.Equal(t, tagID, int64(0), "GetTag returned wrong ID for 'nonexistent' id: %d", tagID)
}

func TestCreateTag(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tx, err := r.DB.Beginx()
	assert.NoError(t, err, "failed to start transaction")
	defer func() {
		assert.NoError(t, tx.Rollback(), "failed to rollback transaction")
	}()

	ttags := r.Cfg.Tables.Tags
	// Test creating a new tag
	tagID, err := r.createTag(tx, ttags, "newtag")
	assert.NoError(t, err, "failed to create tag")
	assert.NotEqual(t, tagID, int64(0), "CreateTag returned ID 0, expected a valid ID")

	// Verify tag was inserted
	var insertedTagID int64
	var tagName string
	q := fmt.Sprintf("SELECT id, name FROM %s WHERE id = ?", ttags)
	err = tx.QueryRow(q, tagID).Scan(&insertedTagID, &tagName)
	assert.NoError(t, err, "failed to query inserted tag")
	assert.Equal(t, tagName, "newtag", "tag name should be 'newtag'")

	// test creating a duplicate tag (assuming unique constraint on name)
	_, err = r.createTag(tx, ttags, "newtag")
	assert.Error(t, err, "CreateTag should have failed for duplicate tag")
}
