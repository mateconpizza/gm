package git

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

type (
	Readme struct {
		Enabled bool   `json:"enabled" yaml:"enabled"`
		Glyphs  string `json:"glyphs"  yaml:"glyphs"`
		Summary string `json:"summary" yaml:"summary"`
	}

	Config struct {
		Enabled bool    `json:"enabled"          yaml:"enabled"`          // Enable git
		Log     bool    `json:"logging"          yaml:"logging"`          // Enable logging
		Remote  string  `json:"remote,omitempty" yaml:"remote,omitempty"` // Remote repo
		Readme  *Readme `json:"readme,omitempty" yaml:"readme,omitempty"` // Readme repo
		Hooks   []Hook  `json:"-"                yaml:"-"`

		writer io.Writer // Writer for logging
	}

	ConfigOpt func(*Config)
)

func NewConfig(opts ...ConfigOpt) *Config {
	c := &Config{
		Log:    true,
		writer: os.Stdout,
		Readme: &Readme{},
	}

	for _, fn := range opts {
		fn(c)
	}

	return c
}

func WithConfigWriter(w io.Writer) ConfigOpt { return func(c *Config) { c.writer = w } }
func WithConfigGlyphs(g string) ConfigOpt    { return func(c *Config) { c.Readme.Glyphs = g } }
func WithConfigSummary(s string) ConfigOpt   { return func(c *Config) { c.Readme.Summary = s } }

func (c *Config) SetWriter(w io.Writer) { c.writer = w }
func (c *Config) Writer() io.Writer     { return c.writer }

func (c *Config) Status() string {
	if !c.Enabled {
		return "disabled"
	}
	return "enabled"
}

func (c *Config) Logging() string {
	if !c.Log {
		return "disabled"
	}
	return "enabled"
}

func (c *Config) Load() {
	if !c.Log {
		c.writer = io.Discard
	}
	slog.Info("git initialized",
		"log_enabled", c.Log,
		"writer", fmt.Sprintf("%T", c.writer),
	)
}
