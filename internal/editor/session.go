package editor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

type Meta struct {
	DBName  string
	Version string
}

type postRunEditionFunc func(original, updated *Record) error

type SessionOption func(*EditSession)

// EditSession build -> edit -> parse -> confirm -> save.
type EditSession struct {
	Console     *ui.Console
	Editor      *TextEditor
	DB          *db.SQLite
	postEdition postRunEditionFunc
	ctx         context.Context
	filetype    string
	meta        *Meta
}

func WithPostEditionRunE(fn postRunEditionFunc) SessionOption {
	return func(es *EditSession) {
		es.postEdition = fn
	}
}

func WithContext(ctx context.Context) SessionOption {
	return func(es *EditSession) {
		es.ctx = ctx
	}
}

func WithFileType(ft string) SessionOption {
	return func(es *EditSession) {
		// TODO: add `FileType()` method to Strategy
		es.filetype = ft
	}
}

func WithMeta(m *Meta) SessionOption {
	return func(es *EditSession) {
		es.meta = m
	}
}

// Run processes records for editing using the specified strategy.
func (e *EditSession) Run(bs []*Record, strategy EditStrategy) error {
	n := len(bs)
	for i, b := range bs {
		if err := e.processSingleRecord(b, i, n, strategy); err != nil {
			return err
		}
	}
	return nil
}

// processSingleRecord handles the edit loop for a single record.
func (e *EditSession) processSingleRecord(original *Record, idx, total int, strategy EditStrategy) error {
	currentRecord := original

	// Loop to handle the "retry" action for a single record.
	for {
		editedBuf, err := e.buildAndEdit(currentRecord, idx, total, strategy)
		if err != nil {
			return err
		}

		updated, err := strategy.ParseBuffer(e.ctx, editedBuf, currentRecord, idx, total)
		if errors.Is(err, ErrBufferUnchanged) {
			return nil // Success: nothing changed, move to the next record.
		}
		if err != nil {
			return err
		}

		e.Console.Frame().Reset().Header("Diff:\n").Flush()
		fmt.Println(strategy.Diff(original, updated))

		opt, err := e.Console.Choose("save changes?", []string{"yes", "no", "edit"}, "y")
		if err != nil {
			return err
		}

		switch strings.ToLower(opt) {
		case "y", "yes":
			return e.saveRecordChanges(strategy, original, updated)
		case "n", "no":
			// Skip and continue
			return nil
		case "e", "edit":
			// Retry
			currentRecord = updated
		}
	}
}

// buildAndEdit prepares record for editing and launches editor.
func (e *EditSession) buildAndEdit(r *Record, idx, total int, s EditStrategy) ([]byte, error) {
	buf, err := s.BuildBuffer(e.meta, r, idx, total)
	if err != nil {
		return nil, err
	}
	return e.Editor.Bytes(e.ctx, buf, e.filetype)
}

// saveRecordChanges persists updated record to database.
func (e *EditSession) saveRecordChanges(strategy EditStrategy, original, updated *Record) error {
	if err := strategy.Save(e.ctx, e.DB, updated); err != nil {
		return err
	}

	if e.postEdition != nil {
		if err := e.postEdition(original, updated); err != nil {
			return err
		}
	}

	fmt.Print(e.Console.SuccessMesg(fmt.Sprintf("bookmark [%d] changes saved\n", updated.ID)))
	return nil
}

func NewMeta(s, ver string) *Meta {
	return &Meta{DBName: s, Version: ver}
}

// NewEditSession creates a new editing session.
func NewEditSession(c *ui.Console, r *db.SQLite, e *TextEditor, opts ...SessionOption) *EditSession {
	s := &EditSession{
		Console: c,
		Editor:  e,
		DB:      r,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.meta == nil {
		s.meta = NewMeta("dbname?", "0.0.1")
	}

	if s.ctx == nil {
		s.ctx = context.Background()
	}

	return s
}
