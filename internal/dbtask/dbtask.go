// Package dbtask provides functions for managing SQLite databases.
package dbtask

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrBackupExists   = errors.New("backup already exists")
	ErrDBCorrupted    = errors.New("database corrupted")
	ErrRecordNotFound = errors.New("no record found")
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

// DropFromPath drops the database from the given path.
func DropFromPath(ctx context.Context, dbPath string) error {
	r, err := db.New(dbPath)
	if err != nil {
		return err
	}
	return r.DropSecure(ctx)
}

// TagsCounterFromPath returns a map with tag as key and count as value.
func TagsCounterFromPath(ctx context.Context, dbPath string) (map[string]int, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer r.Close()

	return r.TagsCounter(ctx)
}

func NewRepoStats(ctx context.Context, dbPath string) (*RepoStats, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	return &RepoStats{
		Name:      r.Name(),
		Bookmarks: r.Count(ctx, "bookmarks"),
		Tags:      r.Count(ctx, "tags"),
		Favorites: r.CountFavorites(ctx),
		Size:      files.SizeFormatted(r.Cfg.Fullpath()),
	}, nil
}
