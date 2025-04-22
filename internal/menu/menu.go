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
	ErrFzfNoRecords   = errors.New("fzf: no records provided")
	ErrFzfReturnCode  = errors.New("fzf: returned a non-zero code")
)

type OptFn func(*Options)

type Options struct {
	keybind     []string
	header      []string
	settings    FzfSettings
	defaults    bool
	interruptFn func(error)
}

type Menu[T comparable] struct {
	Options
}

// AddOpts adds options to the menu.
func (m *Menu[T]) AddOpts(opts ...OptFn) {
	for _, fn := range opts {
		fn(&m.Options)
	}
}

// defaultOpts returns the default options.
func defaultOpts() Options {
	return Options{
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
	}
}

// callInterruptFn calls the interrupt function.
func (m *Menu[T]) callInterruptFn(err error) {
	if m.interruptFn != nil {
		slog.Debug("calling interruptFn with err", "err", err)
		m.interruptFn(err)
	}

	slog.Warn("interruptFn is nil")
}

// SetInterruptFn sets the interrupt function for the menu.
func (m *Menu[T]) SetInterruptFn(fn func(error)) {
	m.interruptFn = fn
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

// WithPreview adds preview with a custom command.
func WithPreview(cmd string) OptFn {
	return buildPreviewOpts(cmd)
}

// buildPreviewOpts builds the preview options.
func buildPreviewOpts(cmd string) OptFn {
	preview := menuConfig.Keymaps.Preview
	if !preview.Enabled {
		return func(o *Options) {}
	}

	var opts []string
	if !colorEnabled {
		opts = append(opts, "--no-color")
	}
	opts = append(opts, "--preview="+cmd)
	if !menuConfig.Preview {
		opts = append(opts, "--preview-window=hidden,up")
	} else {
		opts = append(opts, "--preview-window=~4,+{2}+4/3,<80(up)")
	}

	return func(o *Options) {
		o.settings = append(o.settings, opts...)
		if !preview.Hidden && menuConfig.Preview {
			o.header = appendKeytoHeader(o.header, preview.Bind, "toggle-preview")
		}
		o.keybind = append(o.keybind, preview.Bind+":toggle-preview")
	}
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
	o := defaultOpts()
	for _, fn := range opts {
		fn(&o)
	}

	return &Menu[T]{
		Options: o,
	}
}

// setup loads header, keybind and args from Options.
func (m *Menu[T]) setup() error {
	loadHeader(m.header, &m.settings)
	return loadKeybind(m.keybind, &m.settings)
}

// Select runs Fzf with the given items and returns the selected item/s.
func (m *Menu[T]) Select(items []T, preprocessor func(*T) string) ([]T, error) {
	if len(items) == 0 {
		return nil, ErrFzfNoRecords
	}

	if err := m.setup(); err != nil {
		return nil, err
	}

	if preprocessor == nil {
		slog.Warn("preprocessor is nil")
		preprocessor = toString
	}

	slog.Debug("menu args", "args", m.settings)

	// channels
	inputChan := formatItems(items, preprocessor)
	outputChan := make(chan string)
	resultChan := make(chan []T)

	go processOutput(items, preprocessor, outputChan, resultChan)

	// Build Fzf.Options
	options, err := fzf.ParseOptions(m.defaults, m.settings)
	if err != nil {
		return nil, fmt.Errorf("fzf: %w", err)
	}

	// Set up input and output channels
	options.Input = inputChan
	options.Output = outputChan

	// Run Fzf
	retcode, err := fzf.Run(options)
	if retcode != 0 {
		// regardless of what kind of error, always call `callInterruptFn`
		err = handleFzfErr(retcode)
		m.callInterruptFn(err)

		return nil, err
	}

	close(outputChan)
	result := <-resultChan

	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return result, nil
}

// handleFzfErr returns an error based on the exit code of fzf.
//
//	0      Normal exit
//	1      No match
//	2      Error
//	126    Permission denied error from become action
//	127    Invalid shell command for become action.
//	130    Interrupted with CTRL-C or ESC.
func handleFzfErr(retcode int) error {
	switch retcode {
	case 1:
		return ErrFzfNoMatching
	case 2:
		return ErrFzf
	case 126:
		return ErrFzfInvalidShellCommand
	case 127:
		return ErrFzfPermissionDenied
	case 130:
		return ErrFzfActionAborted
	}

	return nil
}
