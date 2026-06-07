package db

import (
	"context"
	"fmt"
	"log/slog"
)

type Table string

const (
	TableBookmarks Table = "bookmarks"
	TableTags      Table = "tags"
	TableRelation  Table = "bookmark_tags"
	TableMetadata  Table = "metadata"
)

func (t Table) Exists(ctx context.Context, r *SQLite) (bool, error) {
	var count int
	err := r.DB.GetContext(
		ctx,
		&count,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?",
		t.String(),
	)
	if err != nil {
		slog.DebugContext(ctx, "checking if table exists", "name", t, "error", err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}

func (t Table) String() string {
	return string(t)
}

var tables = []Table{
	TableBookmarks,
	TableTags,
	TableRelation,
	TableMetadata,
}

// Init initializes a new database and creates the required tables.
func (r *SQLite) Init(ctx context.Context) error {
	ms, err := LoadMigrations()
	if err != nil {
		return err
	}

	return Migrate(ctx, r, ms)
}

// Vacuum rebuilds the database file, repacking it into a minimal amount of
// disk space.
func (r *SQLite) Vacuum(ctx context.Context) error {
	return vacuum(ctx, r)
}

// DropSecure removes all records database.
func (r *SQLite) DropSecure(ctx context.Context) error {
	return drop(ctx, r)
}

// IsInitialized reports whether the database schema has been initialized.
func (r *SQLite) IsInitialized(ctx context.Context) bool {
	v, err := CurrentSchemaVersion(ctx, r)
	if err != nil {
		slog.DebugContext(ctx, "getting schema version", "error", err)
		return false
	}

	return v > 0
}
