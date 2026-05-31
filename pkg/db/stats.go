package db

import (
	"context"
	"fmt"
	"strings"
)

// RepoStats holds metadata about a bookmark repository.
type RepoStats struct {
	Name        string `db:"-"`
	Bookmarks   int    `db:"total_bookmarks"`
	Tags        int    `db:"total_tags"`
	Favorites   int    `db:"favorites"`
	Archived    int    `db:"archived"`
	DeadLinks   int    `db:"dead_links"`
	TotalVisits int    `db:"total_visits"`
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

func NewStats(ctx context.Context, r *SQLite) (*RepoStats, error) {
	var stats RepoStats
	err := r.DB.GetContext(ctx, &stats, `SELECT * FROM stats`)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}

	return &stats, nil
}

func (r *SQLite) Stats(ctx context.Context, dest any) error {
	return r.DB.GetContext(ctx, dest, `SELECT * FROM stats`)
}
