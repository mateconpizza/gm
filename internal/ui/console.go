// Package ui provides console interaction utilities with styled output.
// It wraps terminal operations with colored frames and user prompts.
package ui

import (
	"context"
	"io"
	"os"

	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
)

type Console struct {
	term    *terminal.Term
	frame   *frame.Frame
	palette *ansi.Palette
	writer  io.Writer
}

// Option is a function type for configuring Console.
type Option func(*Console)

// NewConsole creates a new Console with the given options.
func NewConsole(opts ...Option) *Console {
	c := &Console{palette: ansi.NewPalette()}
	for _, opt := range opts {
		opt(c)
	}

	if c.term == nil {
		c.term = terminal.New()
	}

	if c.frame == nil {
		c.frame = frame.New()
	}

	if c.writer == nil {
		c.writer = os.Stdout
	}

	return c
}

func NewDefaultConsole(ctx context.Context, f func(error)) *Console {
	return NewConsole(
		WithFrame(frame.New(
			frame.WithColorBorder(ansi.BrightBlack),
			frame.WithIcons(&frame.Icons{
				Error:    frame.IconStyle{Symbol: "✗", Color: ansi.BrightRed.With(ansi.Bold)},
				Warning:  frame.IconStyle{Symbol: "!", Color: ansi.BrightYellow.With(ansi.Bold)},
				Info:     frame.IconStyle{Symbol: "i", Color: ansi.BrightBlue.With(ansi.Bold)},
				Question: frame.IconStyle{Symbol: "?", Color: ansi.BrightGreen.With(ansi.Bold)},
				Success:  frame.IconStyle{Symbol: "✓", Color: ansi.BrightGreen.With(ansi.Bold)},
			}),
		)),
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

func WithWriter(w io.Writer) Option {
	return func(c *Console) {
		c.writer = w
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
func (c *Console) Palette() *ansi.Palette       { return c.palette }
func (c *Console) Writer() io.Writer            { return c.writer }
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
	success := c.palette.BrightGreen.Wrap("Successfully ", c.palette.Italic)
	mesg := c.palette.Italic.Sprint(a...)
	return c.frame.Reset().Success(success + mesg).StringReset()
}

// ErrorMesg returns a prettified error message.
func (c *Console) ErrorMesg(a ...any) string {
	err := c.palette.BrightRed.Wrap("Error ", c.palette.Italic)
	mesg := c.palette.Italic.Sprint(a...)
	return c.frame.Reset().Error(err + mesg).StringReset()
}

// WarningMesg returns a prettified warning message.
func (c *Console) WarningMesg(a ...any) string {
	wanr := c.palette.BrightYellow.Wrap("Warning ", c.palette.Italic)
	mesg := c.palette.Italic.Sprint(a...)
	return c.frame.Reset().Warning(wanr + mesg).StringReset()
}

// InfoMesg returns a prettified info message.
func (c *Console) InfoMesg(a ...any) string {
	info := c.palette.BrightBlue.Wrap("Info ", c.palette.Italic)
	mesg := c.palette.Italic.Sprint(a...)
	return c.frame.Reset().Info(info + mesg).StringReset()
}

func (c *Console) Error(s string) *frame.Frame   { return c.frame.Reset().Error(s) }
func (c *Console) Info(s string) *frame.Frame    { return c.frame.Reset().Info(s) }
func (c *Console) Success(s string) *frame.Frame { return c.frame.Reset().Success(s) }
func (c *Console) Warning(s string) *frame.Frame { return c.frame.Reset().Warning(s) }
func (c *Console) Flush() *frame.Frame           { return c.frame.Flush() }
func (c *Console) Reset() *frame.Frame           { return c.frame.Reset() }
