// Package menu provides a flexible wrapper for the fzf interactive filter,
// enabling customizable selection menus.
package menu

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

var (
	// fzf errors.
	ErrFzf                    = errors.New("fzf: error: code 2")
	ErrFzfNoMatching          = errors.New("fzf: no matching record: code 1")
	ErrFzfInvalidShellCommand = errors.New("fzf: invalid shell command for become action: code 126")
	ErrFzfActionAborted       = errors.New("fzf: action aborted: code 130")
	ErrFzfPermissionDenied    = errors.New("fzf: permission denied from become action: code 127")

	// menu errors.
	ErrFzfExitError   = errors.New("fzf: exit error")
	ErrFzfInterrupted = errors.New("fzf: returned exit code 130")
	ErrFzfNoItems     = errors.New("fzf: no items found")
	ErrFzfReturnCode  = errors.New("fzf: returned a non-zero code")
)

// Border draw border around the finder.
type Border string

const (
	BorderRounded    Border = "rounded"
	BorderSharp      Border = "sharp"
	BorderBold       Border = "bold"
	BorderDouble     Border = "double"
	BorderBlock      Border = "block"
	BorderThinblock  Border = "thinblock"
	BorderHorizontal Border = "horizontal"
	BorderVertical   Border = "vertical"
	BorderLine       Border = "line"
	BorderTop        Border = "top"
	BorderBottom     Border = "bottom"
	BorderLeft       Border = "left"
	BorderRight      Border = "right"
	BorderNone       Border = "none"
)

func (b Border) Arg(s string) string {
	return s + "=" + string(b)
}

type Option func(*Options)

type Options struct {
	// header contains header lines displayed in the FZF interface.
	// When customHeaderOnly is false, keymap descriptions are appended.
	header []string

	// customHeaderOnly indicates whether to use only custom headers.
	// When true, keymap descriptions are excluded from the header.
	customHeaderOnly bool

	// arg holds the command-line arguments passed to FZF.
	// These are built from various options and configurations.
	args *ArgsBuilder

	// interruptFn handles FZF cancellation signals (Ctrl-C, ESC, etc.).
	interruptFn func(error)

	// runner executes the FZF command and handles I/O.
	// Can be customized for testing or different execution environments.
	runner MenuRunner

	// keymaps manages the keyboard shortcuts and their actions.
	// Provides methods to register and manage keybindings.
	keymaps *keyManager

	// cfg contains the menu configuration and styling options.
	// Provides defaults and behavioral settings for the menu.
	cfg *Config

	// previewCmd specifies the command for FZF's preview window.
	// This command is executed for each item to generate preview content.
	// Example: "bat --color=always {}"
	previewCmd string

	// enable output color
	withOutputColor bool

	// multi enable multi-select with tab/shift-tab.
	multi bool
}

// Items holds the data and transformation logic for menu items.
type Items[T comparable] struct {
	// Formatter converts items to display strings for FZF.
	// If nil, a default Formatter will be used that calls String() method.
	// The function should return ANSI-formatted strings for rich display.
	Formatter func(*T) string
}

type Menu[T comparable] struct {
	Options
	Items[T]
}

// Select executes Fzf with the set elements and returns the selected item/s.
func (m *Menu[T]) Select(items []T) ([]T, error) {
	if err := m.buildArgs(); err != nil {
		return nil, err
	}

	selected, err := selectFromItems(m, items)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, ErrFzfNoItems
	}

	return selected, nil
}

// AddOpts adds options to the menu.
func (m *Menu[T]) AddOpts(opts ...Option) {
	for _, fn := range opts {
		fn(&m.Options)
	}
}

// callInterruptFn safely executes the interrupt callback if set.
func (m *Menu[T]) callInterruptFn(err error) {
	if m.interruptFn != nil {
		slog.Debug("calling interruptFn with err", "err", err)
		m.interruptFn(err)
	}

	slog.Debug("interruptFn is nil")
}

// SetInterruptFn sets the interrupt function for the menu.
func (m *Menu[T]) SetInterruptFn(fn func(error)) {
	m.interruptFn = fn
}

// SetFormatter sets a function to format items for display in fzf.
func (m *Menu[T]) SetFormatter(preprocessor func(*T) string) {
	m.Formatter = preprocessor
}

func (m *Menu[T]) UpdatePreview(s string) {
	m.previewCmd = s
}

// WithInterruptFn sets a callback that executes on fzf interruption.
// Use for cleanup or custom error handling when user cancels selection.
func WithInterruptFn(fn func(error)) Option {
	return func(o *Options) {
		o.interruptFn = fn
	}
}

// WithArgs adds new args to Fzf.
func WithArgs(args ...string) Option {
	return func(o *Options) {
		o.args.add(args...)
	}
}

func WithConfig(c *Config) Option {
	return func(o *Options) {
		o.cfg = c
		o.args.add(c.Arguments...)
	}
}

// WithKeybinds adds a keybind to Fzf.
func WithKeybinds(keys ...*Keymap) Option {
	return func(o *Options) {
		o.keymaps.register(keys...)
	}
}

// WithMultiSelection adds a keybind to select multiple records.
func WithMultiSelection() Option {
	return func(o *Options) {
		o.multi = true
	}
}

// WithRunner Add new OptionFn for test configuration.
func WithRunner(r MenuRunner) Option {
	return func(o *Options) {
		o.runner = r
	}
}

// WithPreview adds preview with a custom command.
func WithPreview(cmd string) Option {
	return func(o *Options) {
		o.previewCmd = cmd
	}
}

// WithMultilineView adds multiline view and highlights the entire current line
// in Fzf.
func WithMultilineView() Option {
	return func(o *Options) {
		o.args.add(o.args.highlightLine, o.args.read0)
	}
}

// WithHeader adds a header to FZF, appending to existing headers.
func WithHeader(header string) Option {
	return func(o *Options) {
		o.header = append([]string{header}, o.header...)
	}
}

// WithHeaderOnly sets a single header, replacing all existing ones.
func WithHeaderOnly(header string) Option {
	return func(o *Options) {
		o.header = []string{header}
		o.customHeaderOnly = true
	}
}

// WithHeaderFirst print header before the prompt line.
func WithHeaderFirst() Option {
	return func(o *Options) {
		o.args.add(o.args.headerFirst)
	}
}

// WithHeaderBorder draw border around the header section.
func WithHeaderBorder(b Border) Option {
	return func(o *Options) {
		o.args.add(b.Arg(o.args.headerBorder))
	}
}

// WithHeaderLabel label to print on the header border.
func WithHeaderLabel(s string) Option {
	return func(o *Options) {
		o.args.add(o.args.headerLabel + "=" + s)
	}
}

// WithPreviewBorder draws a single separator line.
func WithPreviewBorder(b Border) Option {
	return func(o *Options) {
		o.args.add(b.Arg(o.args.previewBorder))
	}
}

// WithNth transform the presentation of each line using the field index
// expressions.
func WithNth(idx ...string) Option {
	return func(o *Options) {
		o.args.add(o.args.withNth, strings.Join(idx, ","))
	}
}

func WithFooter(footer string) Option {
	return func(o *Options) {
		o.args.add(o.args.footer + "=" + footer)
	}
}

// WithPrompt adds a prompt to Fzf.
func WithPrompt(s string) Option {
	return func(o *Options) {
		o.args.withPrompt(s)
	}
}

func WithOutputColor(b bool) Option {
	return func(o *Options) {
		o.withOutputColor = b
	}
}

func WithBorderLabel(s string) Option {
	return func(o *Options) {
		o.args.withBorderLabel(s)
	}
}

func WithPointer(s string) Option {
	return func(o *Options) {
		o.args.withPointer(s)
	}
}

// New returns a new Menu.
func New[T comparable](opts ...Option) *Menu[T] {
	o := Options{
		header:  make([]string, 0),
		runner:  &defaultRunner{},
		keymaps: newKeyManager(),
		args:    newArgsBuilder(),
	}

	for _, fn := range opts {
		fn(&o)
	}

	if o.cfg == nil {
		o.cfg = NewDefaultConfig()
	}

	return &Menu[T]{
		Options: o,
	}
}

func (m Menu[T]) Validate() error {
	for _, k := range m.keymaps.list() {
		if !k.Enabled {
			continue
		}

		if k.Bind == "" {
			return fmt.Errorf("%w: empty keybind", ErrInvalidConfigKeymap)
		}
	}

	// set default prompt
	if m.cfg.Prompt == "" {
		slog.Warn("empty prompt, loading default prompt")

		m.cfg.Prompt = defaultPrompt
	}

	// set default header separator
	if m.cfg.Header.Sep == "" {
		slog.Warn("empty header separator, loading default header separator")

		m.cfg.Header.Sep = defaultHeaderSep
	}

	// set default settings
	if len(m.cfg.Arguments) == 0 {
		slog.Warn("empty settings, loading default settings")
	}

	return m.args.Validate()
}
