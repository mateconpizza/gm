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
	app     *application.App
	repo    *db.SQLite
	console *ui.Console
	ctx     context.Context
	writer  io.Writer
}

func WithApplication(app *application.App) Option {
	return func(c *Deps) {
		c.app = app
	}
}

func WithConsole(s *ui.Console) Option {
	return func(c *Deps) {
		c.console = s
	}
}

func WithRepo(r *db.SQLite) Option {
	return func(c *Deps) {
		c.repo = r
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
	if c.app != nil {
		return c.app, nil
	}

	return application.FromContext(c.ctx)
}

func (c *Deps) Repository() (*db.SQLite, error) {
	if c.repo == nil {
		return nil, db.ErrDBNotFound
	}

	return c.repo, nil
}

func (c *Deps) SetRepo(r *db.SQLite)      { c.repo = r }
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
