package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// TagsCounterFromPath returns a map with tag as key and count as value.
func TagsCounterFromPath(dbPath string) (map[string]int, error) {
	r, err := New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return r.TagsCounter()
}

// DropFromPath drops the database from the given path.
func DropFromPath(dbPath string) error {
	r, err := New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return Drop(r, context.Background())
}

// newBackup creates a new backup from the given repository.
func newBackup(r *SQLite) (string, error) {
	// destDSN -> 20060102-150405_dbName.db
	destDSN := fmt.Sprintf("%s_%s", time.Now().Format(r.Cfg.DateFormat), r.Name())
	destPath := filepath.Join(r.Cfg.BackupDir, destDSN)
	slog.Info("creating SQLite backup",
		"src", r.Cfg.Fullpath(),
		"dest", destPath,
	)

	if fileExists(destPath) {
		return "", fmt.Errorf("%w: %q", ErrBackupExists, destPath)
	}

	_ = r.DB.MustExec("VACUUM INTO ?", destPath)

	if err := verifySQLiteIntegrity(destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

// verifySQLiteIntegrity checks the integrity of the SQLite database.
func verifySQLiteIntegrity(path string) error {
	slog.Debug("verifying SQLite integrity", "path", path)

	db, err := openDatabase(path)
	if err != nil {
		return fmt.Errorf("no se pudo abrir backup: %w", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("error closing db", "error", err)
		}
	}()

	var result string
	ctx := context.Background()
	row := db.QueryRowContext(ctx, "PRAGMA integrity_check;")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("%w: %w", ErrDBCorrupted, err)
	}

	if result != "ok" {
		return fmt.Errorf("%w: integrity check: %q", ErrDBCorrupted, result)
	}

	slog.Debug("SQLite integrity verified", "result", result)

	return nil
}

// isInit returns true if the database is initialized.
func isInit(r *SQLite) bool {
	allExist := true

	for _, s := range tablesAndSchema() {
		exists, err := r.tableExists(s.name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.name, "error", err)
			return false
		}

		if !exists {
			allExist = false

			slog.Warn("table does not exist", "name", s.name)
		}
	}

	return allExist
}

// IsInitialized checks if the database is initialized.
func IsInitialized(p string) (bool, error) {
	slog.Debug("checking if database is initialized", "path", p)

	allExist := true

	r, err := New(p)
	if err != nil {
		return false, err
	}

	for _, s := range tablesAndSchema() {
		exists, err := r.tableExists(s.name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.name, "error", err)
			return false, err
		}

		if !exists {
			allExist = false

			slog.Warn("table does not exist", "name", s.name)
		}
	}

	return allExist, nil
}

// Drop removes all records database.
func Drop(r *SQLite, ctx context.Context) error {
	tts := tablesAndSchema()
	tables := make([]Table, 0, len(tts))
	for _, t := range tts {
		tables = append(tables, t.name)
	}

	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteAll(ctx, tables...); err != nil {
			return fmt.Errorf("%w", err)
		}

		return resetSQLiteSequence(ctx, tx, tables...)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.Vacuum(ctx)
}

func vacuum(ctx context.Context, r *SQLite) error {
	slog.Debug("vacuuming database")

	_, err := r.DB.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func resetSQLiteSequence(ctx context.Context, tx *sqlx.Tx, tables ...Table) error {
	if len(tables) == 0 {
		slog.Warn("no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		slog.Debug("resetting sqlite sequence", "table", t)

		if _, err := tx.ExecContext(ctx, "DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			return fmt.Errorf("resetting sqlite sequence: %w", err)
		}
	}

	return nil
}

func RemoveReorder(ctx context.Context, r *SQLite, bs []*BookmarkModel) error {
	// delete records from main table.
	if err := r.DeleteMany(ctx, bs); err != nil {
		return fmt.Errorf("deleting records: %w", err)
	}
	// reorder IDs from main table to avoid gaps.
	if err := r.ReorderIDs(ctx); err != nil {
		return fmt.Errorf("reordering IDs: %w", err)
	}
	// recover space after deletion.
	if err := r.Vacuum(ctx); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// ParseTags normalizes a string of tags by separating them by commas, sorting
// them and ensuring that the final string ends with a comma.
//
//	from: "tag1, tag2, tag3 tag"
//	to: "tag,tag1,tag2,tag3,"
func ParseTags(tags string) string {
	if tags == "" {
		return "notag"
	}

	split := strings.FieldsFunc(tags, func(r rune) bool {
		return r == ',' || r == ' '
	})
	sort.Strings(split)

	tags = strings.Join(uniqueTags(split), ",")
	if strings.HasSuffix(tags, ",") {
		return tags
	}

	return tags + ","
}

// uniqueTags returns a slice of unique tags.
func uniqueTags(t []string) []string {
	var (
		tags []string
		seen = make(map[string]bool)
	)

	for _, tag := range t {
		if tag == "" {
			continue
		}

		if !seen[tag] {
			seen[tag] = true

			tags = append(tags, tag)
		}
	}

	return tags
}

// Validate validates the bookmark.
func Validate(b *BookmarkModel) error {
	if b.URL == "" {
		slog.Error("bookmark is invalid. URL is empty")
		return ErrURLEmpty
	}

	if b.Tags == "," || b.Tags == "" {
		slog.Error("bookmark is invalid. Tags are empty")
		return ErrTagsEmpty
	}

	return nil
}

// fileExists checks if a file exists.
func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

func ensureDBSuffix(s string) string {
	const suffix = ".db"
	if s == "" {
		return s
	}

	e := filepath.Ext(s)
	if e == suffix || e != "" {
		return s
	}

	return fmt.Sprintf("%s%s", s, suffix)
}
