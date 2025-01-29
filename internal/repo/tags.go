package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// GetOrCreateTag returns the tag ID.
func (r *SQLiteRepository) GetOrCreateTag(tx *sqlx.Tx, ttags Table, s string) (int64, error) {
	if s == "" {
		// No tag to process
		return 0, nil
	}

	// try to get the tag within the transaction
	tagID, err := getTag(tx, ttags, s)
	if err != nil {
		return 0, fmt.Errorf("GetOrCreateTag: error retrieving tag: %w", err)
	}

	// if the tag doesn't exist, create it within the transaction
	if tagID == 0 {
		tagID, err = createTag(tx, ttags, s)
		if err != nil {
			return 0, fmt.Errorf("GetOrCreateTag: error creating tag: %w", err)
		}
	}

	return tagID, nil
}

// removeUnusedTags removes unused tags from the database.
func removeUnusedTags(r *SQLiteRepository) error {
	tTags := r.Cfg.Tables.Tags
	tJoin := r.Cfg.Tables.RecordsTags

	return r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		log.Println("starting unused tags cleanup")
		q := fmt.Sprintf(`
    DELETE FROM %s
    WHERE id NOT IN (
        SELECT DISTINCT
            tag_id
        FROM %s)`, tTags, tJoin)
		result, err := tx.Exec(q)
		if err != nil {
			return fmt.Errorf("delete query failed: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			log.Printf("warning: could not get affected rows: %v", err)
			return nil // Non-fatal error
		}

		log.Printf("successfully removed %d unused tags", affected)

		return nil
	})
}

// associateTags associates tags to the given record.
func (r *SQLiteRepository) associateTags(tx *sqlx.Tx, trecords Table, b *Row) error {
	tags := strings.Split(b.Tags, ",")
	log.Printf("associating tags: %v with URL: %s\n", tags, b.URL)
	for _, tag := range tags {
		if tag == "" || tag == " " {
			continue
		}
		tagID, err := r.GetOrCreateTag(tx, r.Cfg.Tables.Tags, tag)
		if err != nil {
			return err
		}
		log.Printf("processing tag: '%s' with id: %d\n", tag, tagID)
		q := fmt.Sprintf("INSERT OR IGNORE INTO %s (bookmark_url, tag_id) VALUES (?, ?)", trecords)
		_ = tx.MustExec(q, b.URL, tagID)
	}

	return nil
}

// updateTags updates the tags associated with the given record.
func (r *SQLiteRepository) updateTags(tx *sqlx.Tx, b *Row) error {
	// delete all tags asossiated with the unique URL
	if err := deleteTags(tx, r.Cfg.Tables.RecordsTags, b.URL); err != nil {
		return fmt.Errorf("updateTags: failed to delete tags: %w", err)
	}
	// add the new tags
	err := r.associateTags(tx, r.Cfg.Tables.RecordsTags, b)
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
		WHERE bt.bookmark_url = ?`
	var tags []string
	err := r.DB.Select(&tags, query, b.URL)
	if err != nil {
		return fmt.Errorf("loadRecordTags: %w: '%w'", ErrRecordScan, err)
	}
	sort.Strings(tags)
	b.Tags = strings.Join(tags, ",")

	return nil
}

// populateTags populates the tags associated with the given records.
func (r *SQLiteRepository) populateTags(bs *Slice) error {
	var bb []Row
	err := bs.ForEachErr(func(b Row) error {
		if err := r.loadRecordTags(&b); err != nil {
			return err
		}
		bb = append(bb, b)

		return nil
	})
	if err != nil {
		return fmt.Errorf("populateTags: %w", err)
	}
	bs.Set(&bb)

	return nil
}

// getTag returns the tag ID.
func getTag(tx *sqlx.Tx, ttags Table, tag string) (int64, error) {
	var tagID int64
	err := tx.QueryRowx(fmt.Sprintf(`SELECT id FROM %s WHERE name = ?`, ttags), tag).Scan(&tagID)
	if errors.Is(err, sql.ErrNoRows) {
		// tag not found
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("getTag: error querying tag: %w", err)
	}

	return tagID, nil
}

// createTag creates a new tag.
func createTag(tx *sqlx.Tx, ttags Table, tag string) (int64, error) {
	result := tx.MustExec(fmt.Sprintf(`INSERT INTO %s (name) VALUES (?)`, ttags), tag)
	tagID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("CreateTag: error getting last insert ID: %w", err)
	}

	return tagID, nil
}

// deleteTags deletes a tag from the given table.
func deleteTags(tx *sqlx.Tx, t Table, bURL string) error {
	log.Printf("deleting tags for URL: %s\n", bURL)
	query := fmt.Sprintf(`DELETE FROM %s WHERE bookmark_url = ?`, t)
	_, err := tx.Exec(query, bURL)
	if err != nil {
		return fmt.Errorf("deleteTags: %w", err)
	}

	return nil
}

// CounterTags returns a map with tag as key and count as value.
func CounterTags(r *SQLiteRepository) (map[string]int, error) {
	query := `
		SELECT t.name, COUNT(bt.tag_id) AS tag_count
		FROM tags t
		LEFT JOIN bookmark_tags bt ON t.id = bt.tag_id
		GROUP BY t.id, t.name;`
	var results []struct {
		Name  string `db:"name"`
		Count int    `db:"tag_count"`
	}
	err := r.DB.Select(&results, query)
	if err != nil {
		return nil, fmt.Errorf("error querying tags count: %w", err)
	}
	tagCounts := make(map[string]int, len(results))
	for _, row := range results {
		tagCounts[row.Name] = row.Count
	}

	return tagCounts, nil
}
