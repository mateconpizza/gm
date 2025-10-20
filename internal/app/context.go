// Package app manages command execution context and shared dependencies.
package app

import (
	"context"

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

func (c *Context) SetDatabase(r *db.SQLite) {
	c.DB = r
}

func (c *Context) Console() *ui.Console {
	return c.console
}

func New(ctx context.Context, opts ...Option) *Context {
	c := &Context{Ctx: ctx}
	for _, opt := range opts {
		opt(c)
	}

	return c
}
