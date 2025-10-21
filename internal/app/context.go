// Package app manages command execution context and shared dependencies.
package app

import (
	"context"
	"io"
	"os"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

type Option func(*Context)

// Context holds the initialized application context.
type Context struct {
	Cfg     *config.Config
	DB      *db.SQLite
	console *ui.Console
	Ctx     context.Context
	writer  io.Writer
}

func WithConfig(a *config.Config) Option {
	return func(c *Context) {
		c.Cfg = a
	}
}

func WithConsole(s *ui.Console) Option {
	return func(c *Context) {
		c.console = s
	}
}

func WithDB(r *db.SQLite) Option {
	return func(c *Context) {
		c.DB = r
	}
}

func WithWriter(w io.Writer) Option {
	return func(c *Context) {
		c.writer = w
	}
}

func (c *Context) SetDatabase(r *db.SQLite) { c.DB = r }
func (c *Context) SetWriter(w io.Writer)    { c.writer = w }
func (c *Context) SetConsole(uc *ui.Console) { c.console = uc }
func (c *Context) Console() *ui.Console     { return c.console }
func (c *Context) Writer() io.Writer        { return c.writer }

func New(ctx context.Context, opts ...Option) *Context {
	c := &Context{Ctx: ctx}
	for _, opt := range opts {
		opt(c)
	}

	if c.writer == nil {
		c.writer = os.Stdout
	}

	return c
}
