// Package ui provides console interaction utilities with styled output.
// It wraps terminal operations with colored frames and user prompts.
package ui

import (
	"context"
	"io"
	"os"

	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
)

var DefaultConsole = NewDefaultConsole(true, func(err error) { sys.ErrAndExit(err) })

type Console struct {
	term    *terminal.Term
	frame   *frame.Frame
	palette *ansi.Palette
	writer  io.Writer

	differ       *Differ
	colorEnabled bool
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

func NewDefaultConsole(withColor bool, fn func(error)) *Console {
	c := NewConsole(
		WithColor(withColor),
		WithDefaultTerminal(withColor, fn),
	)

	p := c.Palette()
	frameOpts := []frame.OptFn{
		frame.WithColorBorder(p.Gray.Sprint),
	}

	if withColor {
		p := c.Palette()
		frameOpts = append(frameOpts,
			frame.WithIcons(&frame.Icons{
				Error:    frame.IconStyle{Symbol: "✗", Color: p.BrightRed.Sprint},
				Warning:  frame.IconStyle{Symbol: "!", Color: p.BrightYellow.Sprint},
				Info:     frame.IconStyle{Symbol: "i", Color: p.BrightBlue.Sprint},
				Question: frame.IconStyle{Symbol: "?", Color: p.BrightGreen.Sprint},
				Success:  frame.IconStyle{Symbol: "✓", Color: p.BrightGreen.Sprint},
			}),
		)
	}
	c.frame = frame.New(frameOpts...)

	return c
}

func WithColor(enabled bool) Option        { return func(c *Console) { c.colorEnabled = enabled } }
func WithFrame(f *frame.Frame) Option      { return func(c *Console) { c.frame = f } }
func WithTerminal(t *terminal.Term) Option { return func(c *Console) { c.term = t } }
func WithWriter(w io.Writer) Option        { return func(c *Console) { c.writer = w } }

func (c *Console) Term() *terminal.Term                      { return c.term }
func (c *Console) Frame() *frame.Frame                       { return c.frame }
func (c *Console) Palette() *ansi.Palette                    { return c.palette }
func (c *Console) Differ() *Differ                           { return c.differ }
func (c *Console) Writer() io.Writer                         { return c.writer }
func (c *Console) IsPiped() bool                             { return c.term.IsPiped() }
func (c *Console) ReplaceLine(s string)                      { c.term.ReplaceLine(1, s) }
func (c *Console) ReplaceLines(n int, s string)              { c.term.ReplaceLine(n, s) }
func (c *Console) SetReader(r io.Reader)                     { c.term.SetReader(r) }
func (c *Console) SetWriter(w io.Writer)                     { c.term.SetWriter(w) }
func (c *Console) Error(s string) *frame.Frame               { return c.frame.Reset().Error(s) }
func (c *Console) Info(s string) *frame.Frame                { return c.frame.Reset().Info(s) }
func (c *Console) Success(s string) *frame.Frame             { return c.frame.Reset().Success(s) }
func (c *Console) Warning(s string) *frame.Frame             { return c.frame.Reset().Warning(s) }
func (c *Console) Flush() *frame.Frame                       { return c.frame.Flush() }
func (c *Console) Reset() *frame.Frame                       { return c.frame.Reset() }
func (c *Console) MaxWidth() int                             { return c.Term().MaxWidth() }
func (c *Console) MinWidth() int                             { return c.Term().MinWidth() }
func (c *Console) Width() int                                { return c.Term().Width() }
func (c *Console) Height() int                               { return c.Term().Height() }
func (c *Console) Print(ctx context.Context, s string) error { return c.Term().Print(ctx, s) }

func WithDefaultTerminal(withColor bool, f func(error)) Option {
	p := ansi.NewPalette()
	cz := terminal.NewColorizer(withColor).
		WithHotkey(p.Red).
		WithError(p.BrightRed.With(p.Bold)).
		WithSuccess(p.BrightGreen.With(p.Bold)).
		WithSelected(p.BrightMagenta.With(p.Bold)).
		WithMuted(p.Dim)

	return WithTerminal(terminal.New(
		terminal.WithInterruptFn(f),
		terminal.WithColorizer(cz),
	))
}

// ConfirmErr prompts the user with a question and options.
func (c *Console) ConfirmErr(ctx context.Context, q, def string) error {
	return c.term.ConfirmErr(ctx, c.frame.Reset().Question(q).StringReset(), def)
}

func (c *Console) Confirm(ctx context.Context, q, def string) bool {
	return c.term.Confirm(ctx, c.frame.Reset().Question(q).StringReset(), def)
}

func (c *Console) ConfirmLimit(ctx context.Context, count, maxItems int, q string, force bool) error {
	if force || count < maxItems {
		return nil
	}
	if !c.Confirm(ctx, q+", continue?", "n") {
		return sys.ErrActionAborted
	}
	c.ReplaceLine(c.Frame().Midln(q).StringReset())
	return nil
}

func (c *Console) Choose(ctx context.Context, q string, opts []string, def string) (string, error) {
	return c.term.Choose(ctx, c.frame.Reset().Question(q).StringReset(), opts, def)
}

func (c *Console) Input(p string) string {
	return c.term.Input(c.frame.Reset().Info(p).StringReset())
}

func (c *Console) InputPassword(ctx context.Context, s string) (string, error) {
	c.frame.Reset().Question(s).Flush()
	return c.term.InputPassword(ctx)
}

func (c *Console) InputPasswordConfirm(ctx context.Context) (string, error) {
	s, err := c.InputPassword(ctx, "Password: ")
	if err != nil {
		return "", err
	}

	if err := c.Print(ctx, "\n"); err != nil {
		return "", err
	}

	s2, err := c.InputPassword(ctx, "Confirm Password: ")
	if err != nil {
		return "", err
	}

	if err := c.Print(ctx, "\n"); err != nil {
		return "", err
	}

	if s != s2 {
		return "", locker.ErrPassphraseMismatch
	}

	return s, nil
}

// Prompt get the input data from the user and return it.
func (c *Console) Prompt(ctx context.Context, p string) (string, error) {
	return c.term.Prompt(ctx, c.frame.Reset().Question(p).StringReset())
}

func (c *Console) PromptWithSuggestions(p string, items []string) string {
	return c.term.PromptWithSuggestions(p, items)
}

func (c *Console) WaitForEnter(ctx context.Context, mesg string) error {
	return c.term.WaitForEnter(ctx, mesg)
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

type Differ struct {
	enabled bool

	muted func(a ...any) string
	add   func(a ...any) string
	del   func(a ...any) string
}

type DifferOpts struct {
	Enabled bool

	Muted func(a ...any) string
	Add   func(a ...any) string
	Del   func(a ...any) string
}

func NewDiffer(opts *DifferOpts) *Differ {
	return &Differ{
		enabled: opts.Enabled,
		muted:   opts.Muted,
		add:     opts.Add,
		del:     opts.Del,
	}
}

func (d *Differ) Added(s string) string   { return d.apply(s, d.add) }
func (d *Differ) Deleted(s string) string { return d.apply(s, d.del) }
func (d *Differ) Muted(s string) string   { return d.apply(s, d.muted) }

func (d *Differ) apply(s string, dc func(a ...any) string) string {
	if !d.enabled || dc == nil {
		return s
	}
	return dc(s)
}

type ColorFunc func(a ...any) string

type BannerConfig struct {
	title        string
	titleColor   ansi.SGR
	comment      string
	subtitle     string
	defaultColor ansi.SGR
	console      *Console
}

func (c *Console) NewBannerBuilder() *BannerConfig {
	return &BannerConfig{
		console:      c,
		defaultColor: c.Palette().White,
	}
}

func (b *BannerConfig) WithConsole(c *Console) *BannerConfig {
	b.console = c
	return b
}

func (b *BannerConfig) WithTitle(s string) *BannerConfig {
	b.title = s
	return b
}

func (b *BannerConfig) WithTitleColor(c ansi.SGR) *BannerConfig {
	b.titleColor = c
	return b
}

func (b *BannerConfig) WithSubtitle(s string) *BannerConfig {
	b.subtitle = s
	return b
}

func (b *BannerConfig) WithComment(s string) *BannerConfig {
	b.comment = s
	return b
}

func (b *BannerConfig) Render() *BannerConfig {
	b.Build().Flush()
	return b
}

func (b *BannerConfig) Build() *frame.Frame {
	p := b.console.Palette()
	title := b.titleColor.Sprint(b.title)
	if b.comment != "" {
		title += p.Dim.With(p.Italic).Sprint(b.comment)
	}
	header := func() string {
		return b.titleColor.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}
	return b.console.
		Frame().
		CustomFunc(header, title).
		Ln().
		Headerln(p.Dim.With(p.Italic).Sprint(b.subtitle))
}
