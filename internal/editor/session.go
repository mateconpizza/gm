package editor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrBufferUnchanged = errors.New("buffer unchanged")
	ErrMissingDep      = errors.New("missing required dependency")
)

type Terminal interface {
	Choose(ctx context.Context, q string, opts []string, def string) (string, error)
	SuccessMesg(a ...any) string
}

type TextEditor interface {
	Edit(ctx context.Context, content []byte, extension string) ([]byte, error)
}

type PersistFunc func(ctx context.Context, old, fresh *bookmark.Bookmark) error

// EditSession build -> edit -> parse -> confirm -> save.
type EditSession struct {
	term     Terminal
	editor   TextEditor
	persist  PersistFunc
	strategy EditStrategy
	writer   io.Writer
	logger   *slog.Logger

	differ Differ
}

func NewEditSession() *EditSession {
	return &EditSession{
		logger: slog.Default(),
		writer: os.Stdout,
	}
}

func (e *EditSession) WithStrategy(es EditStrategy) *EditSession {
	e.strategy = es
	return e
}

func (e *EditSession) WithTerminal(t Terminal) *EditSession {
	e.term = t
	return e
}

func (e *EditSession) WithEditor(te TextEditor) *EditSession {
	e.editor = te
	return e
}

func (e *EditSession) WithWriter(w io.Writer) *EditSession {
	e.writer = w
	return e
}

func (e *EditSession) WithPersistFunc(fn PersistFunc) *EditSession {
	e.persist = fn
	return e
}

func (e *EditSession) WithDiffer(dc Differ) *EditSession {
	e.differ = dc
	return e
}

func (e *EditSession) WithLogger(l *slog.Logger) *EditSession {
	e.logger = l
	return e
}

// Run processes records for editing using the specified strategy.
func (e *EditSession) Run(ctx context.Context, bs []*bookmark.Bookmark) error {
	if err := e.validate(); err != nil {
		return err
	}

	n := len(bs)
	for i, b := range bs {
		if err := e.processSingleRecord(ctx, b, i+1, n); err != nil {
			return err
		}
	}
	return nil
}

// processSingleRecord handles the edit loop for a single record.
func (e *EditSession) processSingleRecord(ctx context.Context, old *bookmark.Bookmark, idx, total int) error {
	log := e.logger.With("record_idx", idx, "total", total, "bookmark_id", old.ID)

	currentRecord := old
	hasPendingChanges := false

	// loop to handle the "retry" action for a single record.
	for {
		log.Debug("starting edit iteration")
		editedBuf, err := e.buildAndEdit(ctx, currentRecord, idx, total, e.strategy)
		if err != nil {
			log.Error("failed during build and edit", "error", err)
			return err
		}

		fresh, err := e.strategy.ParseBuffer(ctx, editedBuf, currentRecord)
		switch {
		case errors.Is(err, ErrBufferUnchanged):
			if !hasPendingChanges {
				log.Debug("buffer unchanged and no pending changes, skipping")
				return nil // success: nothing changed, move to the next record.
			}

			log.Debug("buffer unchanged but has pending changes, preserving previous state")
			// preserve changes from previous editing attempts.
			fresh = currentRecord

		case err != nil:
			log.Error("failed to parse edited buffer", "error", err)
			return err

		default:
			log.Debug("changes detected in edited buffer")
			hasPendingChanges = true
		}

		fmt.Fprintln(e.writer, e.strategy.Diff(e.differ, old, fresh))

		opt, err := e.term.Choose(ctx, "save changes?", []string{"yes", "no", "edit"}, "y")
		if err != nil {
			log.Error("failed to prompt save choice", "error", err)
			return err
		}

		log = log.With("user_choice", opt)

		switch strings.ToLower(opt) {
		case "yes":
			log.Debug("saving record changes")
			return e.saveRecordChanges(ctx, old, fresh)
		case "no":
			log.Debug("user discarded changes")
			// Skip and continue
			return nil
		case "edit":
			log.Debug("user requested re-editing")
			currentRecord = fresh
		}
	}
}

// buildAndEdit prepares record for editing and launches editor.
func (e *EditSession) buildAndEdit(ctx context.Context, r *bookmark.Bookmark, idx, total int, s EditStrategy) ([]byte, error) {
	buf, err := s.BuildBuffer(r, idx, total)
	if err != nil {
		return nil, err
	}
	return e.editor.Edit(ctx, buf, s.FileType())
}

func (e *EditSession) saveRecordChanges(ctx context.Context, original, updated *bookmark.Bookmark) error {
	if err := e.persist(ctx, original, updated); err != nil {
		return err
	}

	fmt.Fprintf(
		e.writer,
		"%s",
		e.term.SuccessMesg(
			fmt.Sprintf("bookmark [%d] changes saved\n", updated.ID),
		),
	)

	return nil
}

func (e *EditSession) validate() error {
	fn := func(opt string) error {
		return fmt.Errorf("%w: call With%s", ErrMissingDep, opt)
	}

	if e.strategy == nil {
		return fn("Strategy")
	}
	if e.term == nil {
		return fn("Terminal")
	}
	if e.editor == nil {
		return fn("Editor")
	}
	if e.persist == nil {
		return fn("PersistFunc")
	}
	if e.differ == nil {
		return fn("Differ")
	}
	return nil
}
