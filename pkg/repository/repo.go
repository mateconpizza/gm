// Package repository implements the repository layer.
package repository

import (
	"context"
	"fmt"

	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// Reader provides methods to read from the repository.
type Reader interface {
	// All returns all bookmarks.
	All() ([]*bookmark.Bookmark, error)

	// ByID returns a bookmark by its ID.
	ByID(id int) (*bookmark.Bookmark, error)

	// ByIDList returns a list of bookmarks by their IDs.
	ByIDList(ids []int) ([]*bookmark.Bookmark, error)

	// ByQuery returns a list of bookmarks by a query.
	ByQuery(query string) ([]*bookmark.Bookmark, error)

	// ByTag returns a list of bookmarks by a tag.
	ByTag(ctx context.Context, tag string) ([]*bookmark.Bookmark, error)

	// Has returns a bookmark by its URL and a boolean indicating if it exists.
	Has(url string) (*bookmark.Bookmark, bool)
}

// Writer provides methods to write, update to the repository.
type Writer interface {
	// InsertOne inserts a bookmark.
	InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error)

	// InsertMany inserts multiple bookmarks.
	InsertMany(ctx context.Context, bs []*bookmark.Bookmark) error

	// DeleteMany deletes multiple bookmarks.
	DeleteMany(ctx context.Context, bs []*bookmark.Bookmark) error

	// Update updates an existing bookmark.
	Update(ctx context.Context, newB, oldB *bookmark.Bookmark) error

	// SetFavorite sets a bookmark as favorite.
	SetFavorite(ctx context.Context, b *bookmark.Bookmark) error

	// AddVisitAndUpdateCount adds a visit to a bookmark and updates its count.
	AddVisitAndUpdateCount(ctx context.Context, bID int) error

	// DeleteReorder deletes records from main table and reorders IDs to avoid
	// gaps.
	DeleteReorder(ctx context.Context, bs []*bookmark.Bookmark) error
}

// Admin provides methods to manage the repository.
type Admin interface {
	// Drop drops the database.
	Drop(ctx context.Context) error

	// Init initializes a new database and creates the required tables.
	Init(ctx context.Context) error

	// Name returns the name of the repository.
	Name() string

	// Fullpath returns the full path to the SQLite database.
	Fullpath() string

	// Close closes the repository.
	Close()
}

// Counter provides methods to count records in the repository.
type Counter interface {
	// Count returns the number of records in the given table.
	Count(table string) int

	// CountFavorites returns the number of favorite records.
	CountFavorites() int

	// CountTags returns tags and their counts.
	CountTags() (map[string]int, error)
}

// Repo is the repository interface.
type Repo interface {
	Admin
	Counter
	Reader
	Writer
}

type sqliteRepo struct {
	store *db.SQLite
}

// New returns a new repository instance.
func New(p string) (Repo, error) {
	store, err := db.New(p)
	if err != nil {
		return nil, err
	}

	return &sqliteRepo{store: store}, nil
}

// NewFromDB returns a new repository instance from a database.
func NewFromDB(r *db.SQLite) Repo {
	if r == nil {
		panic("repo: database cannot be nil")
	}
	return &sqliteRepo{store: r}
}

// Init returns a new initialized repository instance.
func Init(p string) (Repo, error) {
	store, err := db.Init(p)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return &sqliteRepo{store: store}, nil
}

func (r *sqliteRepo) Name() string {
	return r.store.Name()
}

func (r *sqliteRepo) Fullpath() string {
	return r.store.Cfg.Fullpath()
}

func (r *sqliteRepo) InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error) {
	b.GenChecksum()
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

func (r *sqliteRepo) AddVisitAndUpdateCount(ctx context.Context, bID int) error {
	return r.store.AddVisitAndUpdateCount(ctx, bID)
}

func (r *sqliteRepo) InsertMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	dbModels := make([]*db.BookmarkModel, len(bs))
	for i, b := range bs {
		b.GenChecksum()
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
	newB.GenChecksum()
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

func (r *sqliteRepo) Has(url string) (*bookmark.Bookmark, bool) {
	b, ok := r.store.Has(url)
	if !ok {
		return nil, ok
	}
	return FromDBModel(b), ok
}

func (r *sqliteRepo) Drop(ctx context.Context) error {
	return r.store.DropSecure(ctx)
}

func (r *sqliteRepo) Init(ctx context.Context) error {
	return r.store.Init(ctx)
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
		ID:           b.ID,
		URL:          b.URL,
		Tags:         b.Tags,
		Title:        b.Title,
		Desc:         b.Desc,
		CreatedAt:    b.CreatedAt,
		LastVisit:    b.LastVisit,
		UpdatedAt:    b.UpdatedAt,
		VisitCount:   b.VisitCount,
		Favorite:     b.Favorite,
		FaviconURL:   b.FaviconURL,
		FaviconLocal: b.FaviconLocal,
		ArchiveURL:   b.ArchiveURL,
		Checksum:     b.Checksum,
	}
}

// FromDBModel converts a DB model to a domain Bookmark.
func FromDBModel(m *db.BookmarkModel) *bookmark.Bookmark {
	return &bookmark.Bookmark{
		ID:           m.ID,
		URL:          m.URL,
		Tags:         m.Tags,
		Title:        m.Title,
		Desc:         m.Desc,
		CreatedAt:    m.CreatedAt,
		LastVisit:    m.LastVisit,
		UpdatedAt:    m.UpdatedAt,
		VisitCount:   m.VisitCount,
		Favorite:     m.Favorite,
		FaviconURL:   m.FaviconURL,
		FaviconLocal: m.FaviconLocal,
		ArchiveURL:   m.ArchiveURL,
		Checksum:     m.Checksum,
	}
}
