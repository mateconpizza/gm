package db

import (
	"context"
	"fmt"
	"strings"
)

// RepoStats holds metadata about a bookmark repository.
type RepoStats struct {
	Name        string `db:"-"               json:"name"`
	Bookmarks   int    `db:"total_bookmarks" json:"bookmarks"`
	Tags        int    `db:"total_tags"      json:"tags"`
	Favorites   int    `db:"favorites"       json:"favorites"`
	Archived    int    `db:"archived"        json:"archived"`
	DeadLinks   int    `db:"dead_links"      json:"dead_links"`
	TotalVisits int    `db:"total_visits"    json:"total_visits"`
}

func (rs *RepoStats) String() string {
	var parts []string
	if rs.Bookmarks > 0 {
		parts = append(parts, fmt.Sprintf("%d bookmarks", rs.Bookmarks))
	}

	if rs.Tags > 0 {
		parts = append(parts, fmt.Sprintf("%d tags", rs.Tags))
	}

	if rs.Favorites > 0 {
		parts = append(parts, fmt.Sprintf("%d favorites", rs.Favorites))
	}

	if len(parts) == 0 {
		parts = append(parts, "no bookmarks")
	}

	return strings.Join(parts, ", ")
}

func NewStats() *RepoStats {
	return &RepoStats{}
}

func (r *SQLite) Stats(ctx context.Context, dest any) error {
	return r.DB.GetContext(ctx, dest, `SELECT * FROM stats`)
}

func Metadata(r *SQLite, key string) (string, error) {
	type metadata struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}

	var m metadata
	query := `SELECT key, value FROM metadata WHERE key = ?`
	err := r.DB.Get(&m, query, key)
	if err != nil {
		return "", err
	}
	return m.Value, nil
}
