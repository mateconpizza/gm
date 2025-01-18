package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/haaag/gm/internal/slice"
)

// GetOrCreateTag returns the tag ID.
func (r *SQLiteRepository) GetOrCreateTag(tx *sql.Tx, ttags Table, s string) (int64, error) {
	if s == "" {
		// No tag to process
		return 0, nil
	}

	// try to get the tag within the transaction
	tagID, err := r.getTag(tx, ttags, s)
	if err != nil {
		return 0, fmt.Errorf("GetOrCreateTag: error retrieving tag: %w", err)
	}

	// if the tag doesn't exist, create it within the transaction
	if tagID == 0 {
		tagID, err = r.createTag(tx, ttags, s)
		if err != nil {
			return 0, fmt.Errorf("GetOrCreateTag: error creating tag: %w", err)
		}
	}

	return tagID, nil
}

// RemoveUnusedTags removes unused tags from the database.
func (r *SQLiteRepository) RemoveUnusedTags() error {
	log.Println("removing unused tags from the database")

	q := fmt.Sprintf(`
  DELETE FROM
    %s
  WHERE
    id NOT IN (
      SELECT
        DISTINCT tag_id
      FROM
        %s
    )`,
		r.Cfg.Tables.Tags,
		r.Cfg.Tables.RecordsTags,
	)

	result, err := r.DB.Exec(q)
	if err != nil {
		return fmt.Errorf("failed to remove unused tags: %w", err)
	}

	ra, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	log.Printf("removed %d unused tags from the database\n", ra)

	return nil
}

// TagsCounter returns a map with tag as key and count as value.
func TagsCounter(r *SQLiteRepository) (map[string]int, error) {
	query := `
    SELECT t.name, COUNT(bt.tag_id) AS tag_count
    FROM tags t
    LEFT JOIN bookmark_tags bt ON t.id = bt.tag_id
    GROUP BY t.id, t.name;`

	rows, err := r.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying tags count: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on TagCounter: %v", err)
		}
	}()

	tagCounts := make(map[string]int)
	for rows.Next() {
		var tagName string
		var count int
		if err := rows.Scan(&tagName, &count); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		tagCounts[tagName] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return tagCounts, nil
}

// associateTags associates tags to the given record.
func (r *SQLiteRepository) associateTags(tx *sql.Tx, trecords, ttags Table, b *Row) error {
	tags := strings.Split(b.Tags, ",")
	log.Printf("associating tags: %v with URL: %s\n", tags, b.URL)
	for _, tag := range tags {
		if tag == "" || tag == " " {
			continue
		}
		tagID, err := r.GetOrCreateTag(tx, ttags, tag)
		if err != nil {
			return err
		}

		log.Printf("processing tag: '%s' with id: %d\n", tag, tagID)
		q := fmt.Sprintf(
			`INSERT OR IGNORE INTO %s (bookmark_url, tag_id) VALUES (?, ?)`,
			trecords,
		)

		_, err = tx.Exec(q, b.URL, tagID)
		if err != nil {
			return fmt.Errorf("AssociateTags: %w", err)
		}
	}

	return nil
}

// updateTags updates the tags associated with the given record.
func (r *SQLiteRepository) updateTags(tx *sql.Tx, b *Row) error {
	// delete all tags asossiated with the URL
	if err := r.deleteTags(tx, b.URL); err != nil {
		return fmt.Errorf("updateTags: failed to delete tags: %w", err)
	}

	// add the new tags
	err := r.associateTags(tx, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, b)
	if err != nil {
		return fmt.Errorf("updateTags: failed to associate tags: %w", err)
	}

	return nil
}

// loadRecordTags returns the tags associated with the given record.
func (r *SQLiteRepository) loadRecordTags(b *Row) error {
	query := `
			SELECT t.name
			FROM tags t
			JOIN bookmark_tags bt ON t.id = bt.tag_id
			WHERE bt.bookmark_url = ?
		`
	rows, err := r.DB.Query(query, b.URL)
	if err != nil {
		return fmt.Errorf("loadRecordTags: %w: '%w'", ErrRecordScan, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("loadRecordTags: error closing rows on getting record ByID: %v", err)
		}
	}()

	var tags []string
	for rows.Next() {
		var tagName string
		if err := rows.Scan(&tagName); err != nil {
			return fmt.Errorf("loadRecordTags: %w: '%w'", ErrRecordScan, err)
		}
		tags = append(tags, tagName)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("loadRecordTags: %w: '%w'", ErrRecordScan, err)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i] < tags[j]
	})

	b.Tags = strings.Join(tags, ",")

	return nil
}

// populateTags populates the tags associated with the given records.
func (r *SQLiteRepository) populateTags(bs *Slice) error {
	bt := slice.New[Row]()
	getter := func(b Row) error {
		if err := r.loadRecordTags(&b); err != nil {
			return err
		}

		bt.Append(&b)

		return nil
	}

	if err := bs.ForEachErr(getter); err != nil {
		return fmt.Errorf("populateTags: %w", err)
	}

	*bs = *bt

	return nil
}

// getTag returns the tag ID.
func (r *SQLiteRepository) getTag(tx *sql.Tx, ttags Table, tag string) (int64, error) {
	var tagID int64
	query := fmt.Sprintf(`SELECT id FROM %s WHERE name = ?`, ttags)
	err := tx.QueryRow(query, tag).Scan(&tagID)
	if errors.Is(err, sql.ErrNoRows) {
		// Tag not found
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("GetTag: error querying tag: %w", err)
	}

	return tagID, nil
}

// createTag creates a new tag.
func (r *SQLiteRepository) createTag(tx *sql.Tx, ttags Table, tag string) (int64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (name) VALUES (?)`, ttags)
	res, err := tx.Exec(query, tag)
	if err != nil {
		return 0, fmt.Errorf("CreateTag: error inserting tag: %w", err)
	}
	tagID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("CreateTag: error getting last insert ID: %w", err)
	}

	return tagID, nil
}

// deleteTags deletes a tag.
func (r *SQLiteRepository) deleteTags(tx *sql.Tx, bURL string) error {
	log.Printf("deleting tags for URL: %s\n", bURL)
	query := fmt.Sprintf(`DELETE FROM %s WHERE bookmark_url = ?`, r.Cfg.Tables.RecordsTags)
	_, err := tx.Exec(query, bURL)
	if err != nil {
		return fmt.Errorf("deleteTags: error deleting tag: %w", err)
	}

	return nil
}
