// Package dbtask provides functions for managing SQLite databases.
package dbtask

import (
	"context"
	"fmt"

	"github.com/mateconpizza/gm/pkg/db"
)

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
