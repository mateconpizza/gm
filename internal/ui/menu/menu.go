// Package menu provides a flexible wrapper for the fzf interactive filter,
// enabling customizable selection menus.
package menu

import (
	"errors"
	"fmt"
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
	keybind     []string
	header      []string
	settings    FzfSettings
	defaults    bool
	interruptFn func(error)
	runner      MenuRunner
}

type Items[T comparable] struct {
	items        []T
	preprocessor func(*T) string
}

type Menu[T comparable] struct {
	Options
	Items[T]
}

// Select executes Fzf with the set elements and returns the selected item/s.
func (m *Menu[T]) Select() ([]T, error) {
	if err := m.setup(); err != nil {
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

// callInterruptFn calls the interrupt function.
func (m *Menu[T]) callInterruptFn(err error) {
	if m.interruptFn != nil {
		slog.Debug("calling interruptFn with err", "err", err)
		m.interruptFn(err)
	}

	slog.Debug("interruptFn is nil")
}

// setup loads header, keybind and args from Options.
func (m *Menu[T]) setup() error {
	loadHeader(m.header, &m.settings)
	return loadKeybind(m.keybind, &m.settings)
}

// SetInterruptFn sets the interrupt function for the menu.
func (m *Menu[T]) SetInterruptFn(fn func(error)) {
	m.interruptFn = fn
}

// SetItems sets the items for the menu.
func (m *Menu[T]) SetItems(items []T) {
	m.items = items
}

func (m *Menu[T]) SetPreprocessor(preprocessor func(*T) string) {
	m.preprocessor = preprocessor
}

// WithInterruptFn sets the interrupt function for the menu.
func WithInterruptFn(fn func(error)) OptFn {
	return func(o *Options) {
		o.interruptFn = fn
	}
}

// WithArgs adds new args to Fzf.
func WithArgs(args ...string) OptFn {
	return func(o *Options) {
		o.settings = append(o.settings, args...)
	}
}

// WithSettings adds new settings to Fzf.
func WithSettings(settings FzfSettings) OptFn {
	return func(o *Options) {
		o.settings = append(o.settings, settings...)
	}
}

// WithUseDefaults whether to load defaults ($FZF_DEFAULT_OPTS_FILE and
// $FZF_DEFAULT_OPTS).
func WithUseDefaults() OptFn {
	return func(o *Options) {
		o.defaults = true
	}
}

// WithKeybinds adds a keybind to Fzf.
func WithKeybinds(keys ...Keymap) OptFn {
	return func(o *Options) {
		for _, k := range keys {
			if !k.Enabled {
				continue
			}

			if !k.Hidden {
				o.header = appendKeytoHeader(o.header, k.Bind, k.Desc)
			}

			o.keybind = append(o.keybind, fmt.Sprintf("%s:%s", k.Bind, k.Action))
		}
	}
}

// WithMultiSelection adds a keybind to select multiple records.
func WithMultiSelection() OptFn {
	opts := []string{"--highlight-line", "--multi"}

	if !menuConfig.Keymaps.ToggleAll.Enabled {
		return func(o *Options) {
			o.settings = append(o.settings, opts...)
		}
	}

	return func(o *Options) {
		o.settings = append(o.settings, opts...)

		if !menuConfig.Keymaps.ToggleAll.Hidden {
			h := appendKeytoHeader(make([]string, 0), "ctrl-a", "toggle-all")
			o.header = append(o.header, h...)
		}

		o.keybind = append(o.keybind, "ctrl-a:toggle-all")
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
	return buildPreviewOpts(cmd)
}

// WithMultilineView adds multiline view and highlights the entire current line
// in Fzf.
func WithMultilineView() OptFn {
	opts := []string{
		"--highlight-line", // Highlight the whole current line
		"--read0",          // Read input delimited by ASCII NUL characters instead of newline characters
	}

	return func(o *Options) {
		o.settings = append(o.settings, opts...)
	}
}

// WithHeader adds a header to Fzf.
func WithHeader(header string, replace bool) OptFn {
	return func(o *Options) {
		if replace {
			o.header = []string{header}
			return
		}

		o.header = append([]string{header}, o.header...)
	}
}

// WithPrompt adds a prompt to Fzf.
func WithPrompt(prompt string) OptFn {
	return func(o *Options) {
		o.settings = append(o.settings, "--prompt="+prompt)
	}
}

// New returns a new Menu.
func New[T comparable](opts ...OptFn) *Menu[T] {
	defaults := Options{
		settings: []string{
			"--ansi",
			"--reverse",
			"--tac",
			"--height=95%",
			"--info=inline-right",
			"--prompt=" + menuConfig.Prompt,
		},
		defaults: false,
		header:   make([]string, 0),
		runner:   &defaultRunner{},
	}

	for _, fn := range opts {
		fn(&defaults)
	}

	return &Menu[T]{
		Options: defaults,
	}
}

type MenuRunner interface {
	Run(options *fzf.Options) (int, error)
	Parse(defaults bool, settings FzfSettings) (*fzf.Options, error)
}

type defaultRunner struct{}

//nolint:wrapcheck //notneeded
func (d *defaultRunner) Run(options *fzf.Options) (int, error) {
	return fzf.Run(options)
}

//nolint:wrapcheck //notneeded
func (d *defaultRunner) Parse(def bool, s FzfSettings) (*fzf.Options, error) {
	return fzf.ParseOptions(def, s)
}
