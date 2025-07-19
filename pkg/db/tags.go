package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
)

// getOrCreateTag returns the tag ID.
func (r *SQLite) getOrCreateTag(tx *sqlx.Tx, s string) (int64, error) {
	if s == "" {
		// no tag to process
		return 0, nil
	}
	// try to get the tag within the transaction
	tagID, err := getTag(tx, s)
	if err != nil {
		return 0, fmt.Errorf("getting tag: error retrieving tag: %w", err)
	}
	// if the tag doesn't exist, create it within the transaction
	if tagID == 0 {
		tagID, err = createTag(tx, s)
		if err != nil {
			return 0, fmt.Errorf("creating tag: error creating tag: %w", err)
		}
	}

	return tagID, nil
}

// associateTags associates tags to the given record.
func (r *SQLite) associateTags(tx *sqlx.Tx, b *BookmarkModel) error {
	slog.Debug("associating tags with URL", "tags", b.Tags, "url", b.URL)

	tags := strings.SplitSeq(b.Tags, ",")
	for tag := range tags {
		if tag == "" || tag == " " {
			continue
		}

		tagID, err := r.getOrCreateTag(tx, tag)
		if err != nil {
			return err
		}

		slog.Debug("processing Tags", "tag", tag, "tagID", tagID)

		_ = tx.MustExec(
			"INSERT OR IGNORE INTO bookmark_tags (bookmark_url, tag_id) VALUES (?, ?)",
			b.URL,
			tagID,
		)
	}

	return nil
}

// getTag returns the tag ID.
func getTag(tx *sqlx.Tx, tag string) (int64, error) {
	var tagID int64

	err := tx.QueryRowx("SELECT id FROM tags WHERE name = ?", tag).Scan(&tagID)
	if errors.Is(err, sql.ErrNoRows) {
		// tag not found
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("getTag: error querying tag: %w", err)
	}

	return tagID, nil
}

// createTag creates a new tag.
func createTag(tx *sqlx.Tx, tag string) (int64, error) {
	result := tx.MustExec("INSERT INTO tags (name) VALUES (?)", tag)

	tagID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("CreateTag: error getting last insert ID: %w", err)
	}

	return tagID, nil
}

// TagsCounter returns a map with tag as key and count as value.
func (r *SQLite) TagsCounter() (map[string]int, error) {
	q := `
		SELECT
      t.name,
      COUNT(bt.tag_id) AS tag_count
    FROM
      tags t
      LEFT JOIN bookmark_tags bt ON t.id = bt.tag_id
    GROUP BY
      t.id,
      t.name;`

	var results []struct {
		Name  string `db:"name"`
		Count int    `db:"tag_count"`
	}

	if err := r.DB.Select(&results, q); err != nil {
		return nil, fmt.Errorf("error querying tags count: %w", err)
	}

	tagCounts := make(map[string]int, len(results))
	for _, row := range results {
		tagCounts[row.Name] = row.Count
	}

	return tagCounts, nil
}

// TagsList returns the list of tags.
func TagsList(r *SQLite) ([]string, error) {
	var tags []string

	err := r.DB.Select(&tags, `SELECT name FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tags: %w", err)
	}

	return tags, nil
}
