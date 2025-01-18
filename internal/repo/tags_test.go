package repo

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

func TestRemoveUnusedTags(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	// insert test data
	_, err := r.DB.Exec(`INSERT INTO tags (id, name) VALUES (1, 'tag1'), (2, 'tag2'), (3, 'tag3')`)
	if err != nil {
		t.Fatalf("Failed to insert tags: %v", err)
	}

	_, err = r.DB.Exec(
		`INSERT INTO bookmarks (id, url, title, desc) VALUES (1, 'http://example.com', 'Example', 'Description')`,
	)
	if err != nil {
		t.Fatalf("Failed to insert bookmarks: %v", err)
	}

	_, err = r.DB.Exec(`INSERT INTO bookmark_tags (bookmark_url, tag_id) VALUES (1, 1), (1, 2)`)
	if err != nil {
		t.Fatalf("Failed to insert bookmark_tags: %v", err)
	}

	err = r.RemoveUnusedTags()
	if err != nil {
		t.Fatalf("RemoveUnusedTags failed: %v", err)
	}

	rows, err := r.DB.Query(`SELECT id, name FROM tags ORDER BY id`)
	if err != nil {
		t.Fatalf("Failed to query tags: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on TestRemoveUnusedTags: %v", err)
		}
	}()

	if err := rows.Err(); err != nil {
		t.Fatalf("%v: closing rows on getting records by query", err)
	}

	var remainingTags []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		remainingTags = append(remainingTags, name)
	}

	expectedTags := []string{"tag1", "tag2"}
	if !reflect.DeepEqual(remainingTags, expectedTags) {
		t.Errorf("Expected tags %v, but got %v", expectedTags, remainingTags)
	}
}

//nolint:funlen //test
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
		if err != nil {
			t.Fatalf("failed to insert tag: %v", err)
		}
	}

	for _, record := range records {
		_, err := r.DB.Exec("INSERT INTO bookmarks (id, url, title, desc) VALUES (?, ?, ?, ?)",
			record.id, record.url, record.title, record.desc)
		if err != nil {
			t.Fatalf("failed to insert bookmark: %v", err)
		}

		for _, tagID := range record.tagIDs {
			_, err := r.DB.Exec("INSERT INTO bookmark_tags (bookmark_url, tag_id) VALUES (?, ?)",
				record.url, tagID)
			if err != nil {
				t.Fatalf("failed to associate bookmark with tag: %v", err)
			}
		}
	}

	// run tagscounter
	tagCounts, err := CounterTags(r)
	if err != nil {
		t.Fatalf("TagsCounter returned an error: %v", err)
	}

	// expected tag counts
	expectedCounts := map[string]int{
		"tag1": 1, // Used by one record
		"tag2": 2, // Used by two records
		"tag3": 0, // Not used
	}

	// verify results
	for tag, expectedCount := range expectedCounts {
		if count, exists := tagCounts[tag]; !exists || count != expectedCount {
			t.Errorf("tag '%s': expected count %d, got %d", tag, expectedCount, count)
		}
	}

	// ensure no extra tags are in the result
	if len(tagCounts) != len(expectedCounts) {
		t.Errorf("expected %d tags, got %d", len(expectedCounts), len(tagCounts))
	}
}

func TestGetTag(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tx, err := r.DB.Begin()
	if err != nil {
		t.Fatalf("failed to start transaction: %v", err)
	}

	// setup test data
	ttags := r.Cfg.Tables.Tags
	q := fmt.Sprintf("INSERT INTO %s (id, name) VALUES (1, 'tag1'), (2, 'tag2')", ttags)
	_, err = tx.Exec(q)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	// existing tag
	tagID, err := r.getTag(tx, ttags, "tag1")
	if err != nil {
		t.Fatalf("GetTag returned an error for an existing tag: %v", err)
	}
	if tagID != 1 {
		t.Errorf("GetTag returned wrong ID for 'tag1': expected 1, got %d", tagID)
	}

	// non-existent tag
	tagID, err = r.getTag(tx, ttags, "nonexistent")
	if err != nil {
		t.Fatalf("GetTag returned an error for a non-existent tag: %v", err)
	}
	if tagID != 0 {
		t.Errorf("GetTag returned non-zero ID for a non-existent tag: got %d", tagID)
	}
}

func TestCreateTag(t *testing.T) {
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tx, err := r.DB.Begin()
	if err != nil {
		t.Fatalf("failed to start transaction: %v", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			t.Errorf("failed to rollback transaction: %v", err)
		}
	}()

	ttags := r.Cfg.Tables.Tags

	// Test creating a new tag
	tagID, err := r.createTag(tx, ttags, "newtag")
	if err != nil {
		t.Fatalf("CreateTag returned an error: %v", err)
	}
	if tagID == 0 {
		t.Error("CreateTag returned ID 0, expected a valid ID")
	}

	// Verify tag was inserted
	var insertedTagID int64
	var tagName string
	q := fmt.Sprintf("SELECT id, name FROM %s WHERE id = ?", ttags)
	err = tx.QueryRow(q, tagID).
		Scan(&insertedTagID, &tagName)
	if err != nil {
		t.Fatalf("failed to query inserted tag: %v", err)
	}
	if tagName != "newtag" {
		t.Errorf("expected tag name 'newtag', got '%s'", tagName)
	}

	// Test creating a duplicate tag (assuming unique constraint on name)
	_, err = r.createTag(tx, ttags, "newtag")
	if err == nil {
		t.Error("CreateTag did not return an error for a duplicate tag")
	}
}
