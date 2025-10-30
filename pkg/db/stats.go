package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/pkg/files"
)

// RepoStats holds statistics about a bookmark repository.
type RepoStats struct {
	Name      string `json:"dbname"`     // Name is the database base name.
	Bookmarks int    `json:"bookmarks"`  // Bookmarks is the count of bookmarks.
	Tags      int    `json:"tags"`       // Tags is the count of tags.
	Favorites int    `json:"favorites"`  // Favorites is the count of favorite bookmarks.
	Size      string `json:"size_bytes"` // Size in bytes
}

// String returns a string representation of the repo summary.
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

func NewStats(ctx context.Context, dbPath string) (*RepoStats, error) {
	r, err := New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	return r.Stats(ctx), nil
}

func (r *SQLite) Stats(ctx context.Context) *RepoStats {
	return &RepoStats{
		Name:      r.Name(),
		Bookmarks: r.Count(ctx, "bookmarks"),
		Tags:      r.Count(ctx, "tags"),
		Favorites: r.CountFavorites(ctx),
		Size:      files.SizeFormatted(r.Cfg.Fullpath()),
	}
}
