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
func (d *Deps) Context() context.Context {
	return d.ctx
}

// Application retrieves application from context.
func (d *Deps) Application() (*application.App, error) {
	if d.app != nil {
		return d.app, nil
	}

	return application.FromContext(d.ctx)
}

func (d *Deps) Repository() (*db.SQLite, error) {
	if d.repo == nil {
		return nil, db.ErrDBNotFound
	}

	return d.repo, nil
}

func (d *Deps) SetRepo(r *db.SQLite)      { d.repo = r }
func (d *Deps) SetConsole(uc *ui.Console) { d.console = uc }
func (d *Deps) Console() *ui.Console      { return d.console }
func (d *Deps) Writer() io.Writer         { return d.writer }
func (d *Deps) SetWriter(w io.Writer) {
	d.writer = w
	d.console.Frame().SetWriter(w)
	d.console.Term().SetWriter(w)
}

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
