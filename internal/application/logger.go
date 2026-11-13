package application

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mateconpizza/gm/pkg/ansi"
)

func (app *App) SetLogger(l *slog.Logger) {
	app.logger = l
	slog.SetDefault(l)
}

func SetupLogger(verbose int, color bool) *slog.Logger {
	if verbose <= 0 {
		return newLoggerWithColor(slog.LevelError+4, color)
	}

	levels := []slog.Level{
		slog.LevelError, // -v
		slog.LevelWarn,  // -vv
		slog.LevelInfo,  // -vvv
		slog.LevelDebug, // -vvvv
	}

	level := levels[min(verbose, len(levels))-1]
	l := newLoggerWithColor(level, color)

	l.Debug("verbosity", "level", level)

	return l
}

type Handler struct {
	mu     *sync.Mutex // shared between clones so writes don't interleave
	w      io.Writer
	level  slog.Leveler
	color  *ansi.Palette
	prefix string // group prefix, e.g. "http."
	pre    string // pre-rendered attrs from WithAttrs
}

func NewLogger(w io.Writer, level slog.Leveler, p *ansi.Palette) *Handler {
	return &Handler{mu: &sync.Mutex{}, w: w, level: level, color: p}
}

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var sb strings.Builder

	if !r.Time.IsZero() {
		sb.WriteString(h.color.Dim.Sprint(r.Time.Format(time.TimeOnly)))
		sb.WriteByte(' ')
	}

	lvl := r.Level.String()
	sb.WriteString(h.levelStyle(r.Level).Sprint(lvl))
	sb.WriteString(strings.Repeat(" ", max(0, 5-len(lvl))))
	sb.WriteByte(' ')

	h.writeSource(&sb, r.PC)

	sb.WriteString(r.Message)
	sb.WriteString(h.pre)

	r.Attrs(func(a slog.Attr) bool {
		h.writeAttr(&sb, h.prefix, a)
		return true
	})
	sb.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var sb strings.Builder
	sb.WriteString(h.pre)
	for _, a := range attrs {
		h.writeAttr(&sb, h.prefix, a)
	}
	c := *h
	c.pre = sb.String()
	return &c
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	c := *h
	c.prefix += name + "."
	return &c
}

func (h *Handler) levelStyle(l slog.Level) ansi.Style {
	switch {
	case l < slog.LevelInfo:
		return h.color.Gray
	case l < slog.LevelWarn:
		return h.color.Green
	case l < slog.LevelError:
		return h.color.Yellow
	default:
		return h.color.Red.With(h.color.Bold)
	}
}

func (h *Handler) writeAttr(sb *strings.Builder, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}

	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			h.writeAttr(sb, prefix, ga)
		}
		return
	}

	sb.WriteByte(' ')
	sb.WriteString(h.color.Cyan.Sprint(prefix + a.Key))
	sb.WriteString(h.color.Dim.Sprint("="))
	sb.WriteString(h.color.Remover(a.Value.String()))
}

func (h *Handler) writeSource(sb *strings.Builder, pc uintptr) {
	if pc == 0 {
		return
	}
	f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if f.File == "" {
		return
	}

	// "/home/palandri/proj/internal/sys/sys.go" -> "sys/sys.go"
	short := path.Base(path.Dir(f.File)) + "/" + path.Base(f.File)

	sb.WriteString(h.color.Cyan.Sprint("source"))
	sb.WriteString(h.color.Dim.Sprint("="))
	sb.WriteString(h.color.Dim.Sprintf("%s:%d", short, f.Line))
	sb.WriteByte(' ')
}

func newLoggerWithColor(level slog.Level, color bool) *slog.Logger {
	return slog.New(NewLogger(os.Stderr, level, ansi.NewPalette(color)))
}
