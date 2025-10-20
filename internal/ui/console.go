// Package ui provides console interaction utilities with styled output.
// It wraps terminal operations with colored frames and user prompts.
package ui

import (
	"context"
	"io"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

type Console struct {
	term    *terminal.Term
	frame   *frame.Frame
	palette *color.Palette
}

// Option is a function type for configuring Console.
type Option func(*Console)

// NewConsole creates a new Console with the given options.
func NewConsole(opts ...Option) *Console {
	c := &Console{palette: color.NewPalette()}
	for _, opt := range opts {
		opt(c)
	}

	if c.term == nil {
		c.term = terminal.New()
	}

	if c.frame == nil {
		c.frame = frame.New()
	}

	return c
}

func NewDefaultConsole(ctx context.Context, f func(error)) *Console {
	return NewConsole(
		WithFrame(frame.New(frame.WithColorBorder(frame.ColorGray))),
		WithDefaultTerminal(ctx, f),
	)
}

// WithTerminal sets a custom terminal.
func WithTerminal(t *terminal.Term) Option {
	return func(c *Console) {
		c.term = t
	}
}

// WithFrame sets a custom frame.
func WithFrame(f *frame.Frame) Option {
	return func(c *Console) {
		c.frame = f
	}
}

func WithDefaultTerminal(ctx context.Context, f func(error)) Option {
	return WithTerminal(terminal.New(
		terminal.WithContext(ctx),
		terminal.WithInterruptFn(f),
	))
}

func (c *Console) Term() *terminal.Term         { return c.term }
func (c *Console) Frame() *frame.Frame          { return c.frame }
func (c *Console) Palette() *color.Palette      { return c.palette }
func (c *Console) ClearLine(n int)              { c.term.ClearLine(n) }
func (c *Console) ReplaceLine(s string)         { c.term.ReplaceLine(1, s) }
func (c *Console) ReplaceLines(n int, s string) { c.term.ReplaceLine(n, s) }
func (c *Console) SetReader(r io.Reader)        { c.term.SetReader(r) }
func (c *Console) SetWriter(w io.Writer)        { c.term.SetWriter(w) }

// ConfirmErr prompts the user with a question and options.
func (c *Console) ConfirmErr(q, def string) error {
	return c.term.ConfirmErr(c.frame.Reset().Question(q).StringReset(), def)
}

func (c *Console) Confirm(q, def string) bool {
	return c.term.Confirm(c.frame.Reset().Question(q).StringReset(), def)
}

func (c *Console) Choose(q string, opts []string, def string) (string, error) {
	return c.term.Choose(c.frame.Reset().Question(q).StringReset(), opts, def)
}

func (c *Console) Input(p string) string {
	return c.term.Input(c.frame.Reset().Info(p).StringReset())
}

func (c *Console) InputPassword(s string) (string, error) {
	c.frame.Reset().Question(s).Flush()
	return c.term.InputPassword()
}

// Prompt get the input data from the user and return it.
func (c *Console) Prompt(p string) string {
	return c.term.Prompt(c.frame.Reset().Question(p).StringReset())
}

func (c *Console) PromptWithSuggestions(p string, items []string) string {
	return c.term.PromptWithSuggestions(p, items)
}

// SuccessMesg returns a prettified success message.
func (c *Console) SuccessMesg(a ...any) string {
	s := c.palette.BrightGreenItalic("Successfully ") + c.palette.Italic(a...)
	return c.frame.Reset().Success(s).StringReset()
}

// ErrorMesg returns a prettified error message.
func (c *Console) ErrorMesg(a ...any) string {
	s := c.palette.BrightRedItalic("Error ") + c.palette.Italic(a...)
	return c.frame.Reset().Error(s).StringReset()
}

// WarningMesg returns a prettified warning message.
func (c *Console) WarningMesg(a ...any) string {
	s := c.palette.BrightYellowItalic("Warning ") + c.palette.Italic(a...)
	return c.frame.Reset().Warning(s).StringReset()
}

// InfoMesg returns a prettified info message.
func (c *Console) InfoMesg(a ...any) string {
	s := c.palette.BrightBlueItalic("Info ") + c.palette.Italic(a...)
	return c.frame.Reset().Info(s).StringReset()
}

func (c *Console) Error(s string) *frame.Frame   { return c.frame.Reset().Error(s) }
func (c *Console) Info(s string) *frame.Frame    { return c.frame.Reset().Info(s) }
func (c *Console) Success(s string) *frame.Frame { return c.frame.Reset().Success(s) }
func (c *Console) Warning(s string) *frame.Frame { return c.frame.Reset().Warning(s) }
func (c *Console) Flush() *frame.Frame           { return c.frame.Flush() }
func (c *Console) Reset() *frame.Frame           { return c.frame.Reset() }
