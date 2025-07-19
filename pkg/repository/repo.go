// Package repository implements the repository layer.
package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/hasher"
)

type Finder interface {
	All() ([]*bookmark.Bookmark, error)
	ByID(id int) (*bookmark.Bookmark, error)
	ByIDList(ids []int) ([]*bookmark.Bookmark, error)
	ByQuery(query string) ([]*bookmark.Bookmark, error)
	ByURL(ctx context.Context, url string) (*bookmark.Bookmark, error)
	ByTag(ctx context.Context, tag string) ([]*bookmark.Bookmark, error)
	Has(url string) (*bookmark.Bookmark, bool)
}

type Writer interface {
	InsertOne(ctx context.Context, b *bookmark.Bookmark) error
	InsertMany(ctx context.Context, bs []*bookmark.Bookmark) error
	DeleteMany(ctx context.Context, bs []*bookmark.Bookmark) error
	Update(ctx context.Context, newB, oldB *bookmark.Bookmark) error
	SetFavorite(ctx context.Context, b *bookmark.Bookmark) error
	SetVisitDateAndCount(ctx context.Context, b *bookmark.Bookmark) error
	DeleteReorder(ctx context.Context, bs []*bookmark.Bookmark) error
}

type Misc interface {
	Name() string
	Fullpath() string
	Close()
}

type Counter interface {
	Count(table string) int
	CountFavorites() int
	CountTags() (map[string]int, error)
}

type Repo interface {
	Counter
	Finder
	Misc
	Writer
}

type sqliteRepo struct {
	store *db.SQLite
}

func New(r *db.SQLite) Repo {
	return &sqliteRepo{store: r}
}

func (r *sqliteRepo) Name() string {
	return r.store.Name()
}

func (r *sqliteRepo) Fullpath() string {
	return r.store.Cfg.Fullpath()
}

func (r *sqliteRepo) InsertOne(ctx context.Context, b *bookmark.Bookmark) error {
	b.Checksum = hasher.GenChecksum(b.URL, b.Title, b.Desc, b.Tags)
	return r.store.InsertOne(ctx, ToDBModel(b))
}

func (r *sqliteRepo) All() ([]*bookmark.Bookmark, error) {
	dbModels, err := r.store.All()
	if err != nil {
		return nil, err
	}

	// Translate the slice of db models to a slice of domain models.
	bookmarks := make([]*bookmark.Bookmark, len(dbModels))
	for i, m := range dbModels {
		bookmarks[i] = FromDBModel(m)
	}

	return bookmarks, nil
}

func (r *sqliteRepo) SetFavorite(ctx context.Context, b *bookmark.Bookmark) error {
	return r.store.SetFavorite(ctx, ToDBModel(b))
}

func (r *sqliteRepo) SetVisitDateAndCount(ctx context.Context, b *bookmark.Bookmark) error {
	return r.store.SetVisitDateAndCount(ctx, ToDBModel(b))
}

func (r *sqliteRepo) InsertMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	dbModels := make([]*db.BookmarkModel, len(bs))
	for i, b := range bs {
		dbModels[i] = ToDBModel(b)
	}

	return r.store.InsertMany(ctx, dbModels)
}

func (r *sqliteRepo) DeleteMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	dbModels := make([]*db.BookmarkModel, len(bs))
	for i, b := range bs {
		dbModels[i] = ToDBModel(b)
	}

	return r.store.DeleteMany(ctx, dbModels)
}

func (r *sqliteRepo) DeleteReorder(ctx context.Context, bs []*bookmark.Bookmark) error {
	// delete records from main table.
	if err := r.DeleteMany(ctx, bs); err != nil {
		return fmt.Errorf("deleting records: %w", err)
	}
	// reorder IDs from main table to avoid gaps.
	if err := r.store.ReorderIDs(ctx); err != nil {
		return fmt.Errorf("reordering IDs: %w", err)
	}
	// recover space after deletion.
	if err := r.store.Vacuum(ctx); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

func (r *sqliteRepo) Update(ctx context.Context, newB, oldB *bookmark.Bookmark) error {
	newB.Checksum = hasher.GenChecksum(newB.URL, newB.Title, newB.Desc, newB.Tags)
	return r.store.Update(ctx, ToDBModel(newB), ToDBModel(oldB))
}

func (r *sqliteRepo) ByID(id int) (*bookmark.Bookmark, error) {
	b, err := r.store.ByID(id)
	if err != nil {
		return nil, err
	}

	return FromDBModel(b), nil
}

func (r *sqliteRepo) ByIDList(ids []int) ([]*bookmark.Bookmark, error) {
	dbModels, err := r.store.ByIDList(ids)
	if err != nil {
		return nil, err
	}

	// Translate the slice of db models to a slice of domain models.
	bookmarks := make([]*bookmark.Bookmark, len(dbModels))
	for i := range dbModels {
		bookmarks[i] = FromDBModel(&dbModels[i])
	}

	return bookmarks, nil
}

func (r *sqliteRepo) ByURL(ctx context.Context, url string) (*bookmark.Bookmark, error) {
	b, err := r.store.ByURL(url)
	if err != nil {
		return nil, err
	}

	return FromDBModel(b), nil
}

func (r *sqliteRepo) Has(url string) (*bookmark.Bookmark, bool) {
	b, ok := r.store.Has(url)
	if !ok {
		return nil, ok
	}
	return FromDBModel(b), ok
}

func (r *sqliteRepo) ByQuery(query string) ([]*bookmark.Bookmark, error) {
	bs, err := r.store.ByQuery(query)
	if err != nil {
		return nil, err
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		bookmarks = append(bookmarks, FromDBModel(&bs[i]))
	}

	return bookmarks, nil
}

func (r *sqliteRepo) ByTag(ctx context.Context, tag string) ([]*bookmark.Bookmark, error) {
	bs, err := r.store.ByTag(ctx, tag)
	if err != nil {
		return nil, err
	}

	bookmarks := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		bookmarks = append(bookmarks, FromDBModel(&bs[i]))
	}

	return bookmarks, nil
}

func (r *sqliteRepo) Close() {
	r.store.Close()
}

func (r *sqliteRepo) Count(table string) int {
	return r.store.CountRecordsFrom(table)
}

func (r *sqliteRepo) CountFavorites() int {
	return r.store.CountFavorites()
}

func (r *sqliteRepo) CountTags() (map[string]int, error) {
	return r.store.TagsCounter()
}

// ToDBModel converts a domain Bookmark to a DB model.
func ToDBModel(b *bookmark.Bookmark) *db.BookmarkModel {
	return &db.BookmarkModel{
		ID:         b.ID,
		URL:        b.URL,
		Tags:       b.Tags,
		Title:      b.Title,
		Desc:       b.Desc,
		CreatedAt:  b.CreatedAt,
		LastVisit:  b.LastVisit,
		UpdatedAt:  b.UpdatedAt,
		VisitCount: b.VisitCount,
		Favorite:   b.Favorite,
		FaviconURL: b.FaviconURL,
		Checksum:   b.Checksum,
	}
}

// FromDBModel converts a DB model to a domain Bookmark.
func FromDBModel(m *db.BookmarkModel) *bookmark.Bookmark {
	return &bookmark.Bookmark{
		ID:         m.ID,
		URL:        m.URL,
		Tags:       m.Tags,
		Title:      m.Title,
		Desc:       m.Desc,
		CreatedAt:  m.CreatedAt,
		LastVisit:  m.LastVisit,
		UpdatedAt:  m.UpdatedAt,
		VisitCount: m.VisitCount,
		Favorite:   m.Favorite,
		FaviconURL: m.FaviconURL,
		Checksum:   m.Checksum,
	}
}

func NewFromJSON(j *bookmark.BookmarkJSON) *bookmark.Bookmark {
	b := bookmark.New()
	b.ID = j.ID
	b.URL = j.URL
	b.Title = j.Title
	b.Desc = j.Desc
	b.Tags = bookmark.ParseTags(strings.Join(j.Tags, ","))
	b.CreatedAt = j.CreatedAt
	b.LastVisit = j.LastVisit
	b.UpdatedAt = j.UpdatedAt
	b.VisitCount = j.VisitCount
	b.Favorite = j.Favorite
	b.FaviconURL = j.FaviconURL
	b.Checksum = j.Checksum

	return b
}

// ListBackups returns a filtered list of database backups.
func ListBackups(dir, dbName string) ([]string, error) {
	// remove .db|.enc extension for matching
	entries, err := filepath.Glob(filepath.Join(dir, "*_"+dbName+".db*"))
	if err != nil {
		return nil, fmt.Errorf("listing backups: %w", err)
	}

	return entries, nil
}
