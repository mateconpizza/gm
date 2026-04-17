// Package deps manages command execution context and shared dependencies.
package deps

import (
	"context"
	"io"
	"os"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

type Option func(*Deps)

// Deps holds the initialized application context.
type Deps struct {
	App     *application.App
	DB      *db.SQLite
	console *ui.Console
	ctx     context.Context
	writer  io.Writer
}

func WithApplication(app *application.App) Option {
	return func(c *Deps) {
		c.App = app
	}
}

func WithConsole(s *ui.Console) Option {
	return func(c *Deps) {
		c.console = s
	}
}

func WithDB(r *db.SQLite) Option {
	return func(c *Deps) {
		c.DB = r
	}
}

func WithWriter(w io.Writer) Option {
	return func(c *Deps) {
		c.writer = w
	}
}

// Context returns the underlying context.Context.
func (c *Deps) Context() context.Context {
	return c.ctx
}

// Application retrieves application from context.
func (c *Deps) Application() (*application.App, error) {
	return application.FromContext(c.ctx)
}

func (c *Deps) SetDatabase(r *db.SQLite)  { c.DB = r }
func (c *Deps) SetWriter(w io.Writer)     { c.writer = w }
func (c *Deps) SetConsole(uc *ui.Console) { c.console = uc }
func (c *Deps) Console() *ui.Console      { return c.console }
func (c *Deps) Writer() io.Writer         { return c.writer }

func New(ctx context.Context, opts ...Option) *Deps {
	c := &Deps{ctx: ctx}
	for _, opt := range opts {
		opt(c)
	}

	if c.writer == nil {
		c.writer = os.Stdout
	}

	return c
}
