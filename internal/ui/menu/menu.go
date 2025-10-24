// Package menu provides a flexible wrapper for the fzf interactive filter,
// enabling customizable selection menus.
package menu

import (
	"errors"
	"log/slog"

	fzf "github.com/junegunn/fzf/src"
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

type OptFn func(*Options)

type Options struct {
	// header contains header lines displayed in the FZF interface.
	// When customHeaderOnly is false, keymap descriptions are appended.
	header []string

	// customHeaderOnly indicates whether to use only custom headers.
	// When true, keymap descriptions are excluded from the header.
	customHeaderOnly bool

	// arguments holds the command-line arguments passed to FZF.
	// These are built from various options and configurations.
	arguments Args

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
}

// Items holds the data and transformation logic for menu items.
type Items[T comparable] struct {
	// items contains the original data items to be displayed in the menu.
	// These are transformed by the preprocessor for display in FZF.
	items []T

	// preprocessor converts items to display strings for FZF.
	// If nil, a default preprocessor will be used that calls String() method.
	// The function should return ANSI-formatted strings for rich display.
	preprocessor func(*T) string
}

type Menu[T comparable] struct {
	Options
	Items[T]
}

// Select executes Fzf with the set elements and returns the selected item/s.
func (m *Menu[T]) Select() ([]T, error) {
	if err := m.buildArgs(); err != nil {
		return nil, err
	}

	selected, err := selectFromItems(m)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		return nil, ErrFzfNoItems
	}

	return selected, nil
}

// AddOpts adds options to the menu.
func (m *Menu[T]) AddOpts(opts ...OptFn) {
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

// SetItems sets the items for the menu.
func (m *Menu[T]) SetItems(items []T) {
	m.items = items
}

// SetPreprocessor sets a function to format items for display in fzf.
func (m *Menu[T]) SetPreprocessor(preprocessor func(*T) string) {
	m.preprocessor = preprocessor
}

// WithInterruptFn sets a callback that executes on fzf interruption.
// Use for cleanup or custom error handling when user cancels selection.
func WithInterruptFn(fn func(error)) OptFn {
	return func(o *Options) {
		o.interruptFn = fn
	}
}

// WithArgs adds new args to Fzf.
func WithArgs(args ...string) OptFn {
	return func(o *Options) {
		o.arguments = append(o.arguments, args...)
	}
}

func WithConfig(c *Config) OptFn {
	return func(o *Options) {
		o.cfg = c
		o.arguments = append(o.arguments, c.Arguments...)
	}
}

// WithKeybinds adds a keybind to Fzf.
func WithKeybinds(keys ...*Keymap) OptFn {
	return func(o *Options) {
		o.keymaps.register(keys...)
	}
}

// WithMultiSelection adds a keybind to select multiple records.
func WithMultiSelection() OptFn {
	args := []string{
		// Highlight the whole current line
		"--highlight-line",

		// Enable multi-select with tab/shift-tab. It optionally takes an integer
		// argument which denotes the maximum number of items that can be selected.
		"--multi",
	}

	return func(o *Options) {
		o.keymaps.register(&Keymap{
			Bind:    o.cfg.BuiltinKeymaps.ToggleAll.Bind,
			Desc:    "toggle-all",
			Action:  "toggle-all",
			Enabled: o.cfg.BuiltinKeymaps.ToggleAll.Enabled,
			Hidden:  o.cfg.BuiltinKeymaps.ToggleAll.Hidden,
		})
		o.arguments = append(o.arguments, args...)
	}
}

// WithRunner Add new OptionFn for test configuration.
func WithRunner(r MenuRunner) OptFn {
	return func(o *Options) {
		o.runner = r
	}
}

// WithPreview adds preview with a custom command.
func WithPreview(cmd string) OptFn {
	return func(o *Options) {
		o.previewCmd = cmd
	}
}

// WithMultilineView adds multiline view and highlights the entire current line
// in Fzf.
func WithMultilineView() OptFn {
	opts := []string{
		// Highlight the whole current line
		"--highlight-line",

		// Read input delimited by ASCII NUL characters instead of newline characters
		"--read0",
	}

	return func(o *Options) {
		o.arguments = append(o.arguments, opts...)
	}
}

// WithHeader adds a header to FZF, appending to existing headers.
func WithHeader(header string) OptFn {
	return func(o *Options) {
		o.header = append([]string{header}, o.header...)
	}
}

// WithHeaderOnly sets a single header, replacing all existing ones.
func WithHeaderOnly(header string) OptFn {
	return func(o *Options) {
		o.header = []string{header}
		o.customHeaderOnly = true
	}
}

// WithPrompt adds a prompt to Fzf.
func WithPrompt(s string) OptFn {
	return func(o *Options) {
		o.arguments = append(o.arguments, "--prompt="+s)
	}
}

// New returns a new Menu.
func New[T comparable](opts ...OptFn) *Menu[T] {
	o := Options{
		header:  make([]string, 0),
		runner:  &defaultRunner{},
		keymaps: &keyManager{},
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

type MenuRunner interface {
	Run(options *fzf.Options) (int, error)
	Parse(defaults bool, args Args) (*fzf.Options, error)
}

type defaultRunner struct{}

func (d *defaultRunner) Run(options *fzf.Options) (int, error) {
	return fzf.Run(options)
}

func (d *defaultRunner) Parse(def bool, s Args) (*fzf.Options, error) {
	return fzf.ParseOptions(def, s)
}
