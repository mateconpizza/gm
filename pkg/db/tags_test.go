//nolint:wsl,funlen //test
package db

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

func TestTagsCounter(t *testing.T) {
	t.Parallel()

	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	tags := []struct {
		id   int
		name string
	}{
		{1, "tag1"},
		{2, "tag2"},
		{3, "tag3"},
	}

	records := []struct {
		id       int
		url      string
		title    string
		desc     string
		tagIDs   []int
		checksum string
	}{
		{1, "http://example1.com", "Example 1", "Description 1", []int{1, 2}, "checksum"},
		{2, "http://example2.com", "Example 2", "Description 2", []int{2}, "checksum"},
		{3, "http://example3.com", "Example 3", "Description 3", []int{}, "checksum"},
	}

	for _, tag := range tags {
		_, err := r.DB.ExecContext(t.Context(), "INSERT INTO tags (id, name) VALUES (?, ?)", tag.id, tag.name)
		if err != nil {
			t.Fatalf("failed to insert tag %v: %v", tag.name, err)
		}
	}

	for _, record := range records {
		_, err := r.DB.ExecContext(
			t.Context(),
			"INSERT INTO bookmarks (id, url, title, desc, checksum) VALUES (?, ?, ?, ?, ?)",
			record.id,
			record.url,
			record.title,
			record.desc,
			record.checksum,
		)
		if err != nil {
			t.Fatalf("failed to insert bookmark %v: %v", record.url, err)
		}

		for _, tagID := range record.tagIDs {
			_, err := r.DB.ExecContext(
				t.Context(),
				"INSERT INTO bookmark_tags (bookmark_url, tag_id) VALUES (?, ?)",
				record.url,
				tagID,
			)
			if err != nil {
				t.Fatalf("failed to insert bookmark_tag for url %s and tagID %d: %v", record.url, tagID, err)
			}
		}
	}

	tagCounts, err := r.TagsCounter(t.Context())
	if err != nil {
		t.Fatalf("failed to count tags: %v", err)
	}

	expectedCounts := map[string]int{
		"tag1": 1,
		"tag2": 2,
		"tag3": 0,
	}

	for tag, expectedCount := range expectedCounts {
		count, exists := tagCounts[tag]
		if !exists {
			t.Errorf("expected tag %q in results", tag)
			continue
		}
		if count != expectedCount {
			t.Errorf("tag %q: expected count %d, got %d", tag, expectedCount, count)
		}
	}

	if len(tagCounts) != len(expectedCounts) {
		t.Errorf("expected %d tags, got %d", len(expectedCounts), len(tagCounts))
	}
}

func TestGetOrCreateTag(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	newTagName := "newtag"
	err := r.WithTx(t.Context(), func(tx *sqlx.Tx) error {
		tagID, err := createTag(tx, newTagName)
		if err != nil {
			t.Errorf("failed to create tag: %v", err)
		}
		if tagID == 0 {
			t.Errorf("expected non-zero tag ID")
		}

		newTagID, err := getTag(tx, newTagName)
		if err != nil {
			t.Errorf("failed to get tag: %v", err)
		}
		if newTagID != tagID {
			t.Errorf("expected tag ID %d, got %d", tagID, newTagID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}
