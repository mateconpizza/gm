package menu

import (
	"errors"
	"fmt"

	fzf "github.com/junegunn/fzf/src"
)

var (
	ErrFzfExitError       = errors.New("fzf exit error")
	ErrFzfInterrupted     = errors.New("returned exit code 130")
	ErrFzfNoMathching     = errors.New("no matching record")
	ErrFzfNoRecords       = errors.New("no records provided")
	ErrFzfNothingSelected = errors.New("no records selected")
)

var fzfDefaults = []string{
	"--ansi",               // Enable processing of ANSI color codes
	"--cycle",              // Enable cyclic scroll
	"--reverse",            // A synonym for --layout=reverse
	"--sync",               // Synchronous search for multi-staged filtering
	"--info=inline-right",  // Determines the display style of the finder info.
	"--tac",                // Reverse the order of the input
	"--layout=default",     // Choose the layout (default: default)
	"--prompt= Gomarks> ", // Input prompt
	"--no-bold",            // Do not use bold text
}

type OptFn func(*Options)

type Options struct {
	keybind  []string
	header   []string
	args     []string
	defaults bool
}

type Menu[T comparable] struct {
	Options
}

func defaultOpts() Options {
	return Options{
		args:     fzfDefaults,
		defaults: false,
		header:   make([]string, 0),
	}
}

// WithArgs adds new args to Fzf.
func WithArgs(args ...string) OptFn {
	return func(o *Options) {
		o.args = append(o.args, args...)
	}
}

// WithDefaultSettings whether to load defaults ($FZF_DEFAULT_OPTS_FILE and
// $FZF_DEFAULT_OPTS).
func WithDefaultSettings() OptFn {
	return func(o *Options) {
		o.defaults = true
	}
}

// WithKeybindEdit adds a keybind to edit the selected record.
func WithKeybindEdit() OptFn {
	return func(o *Options) {
		o.header = appendKeyDescToHeader(o.header, "ctrl-e", "edit")
		o.keybind = append(o.keybind, withCommand("ctrl-e:execute(%s -e {1})"))
	}
}

// WithKeybindOpen adds a keybind to open the selected record in default
// browser.
func WithKeybindOpen() OptFn {
	return func(o *Options) {
		o.header = appendKeyDescToHeader(o.header, "ctrl-o", "open")
		o.keybind = append(o.keybind, withCommand("ctrl-o:execute(%s -o {1})"))
	}
}

func WithKeybindQR() OptFn {
	return func(o *Options) {
		o.header = appendKeyDescToHeader(o.header, "ctrl-k", "QRcode")
		o.keybind = append(o.keybind, withCommand("ctrl-k:execute(%s --qr --open {1})"))
	}
}

// WithDefaultKeybinds adds default keybinds to Fzf.
//
// ctrl-y:copy-to-clipboard.
func WithDefaultKeybinds() OptFn {
	return func(o *Options) {
		o.header = appendKeyDescToHeader(o.header, "ctrl-y", "copy")
		o.keybind = append(o.keybind, withCommand("ctrl-y:execute(%s -c {1})"))
	}
}

// WithKeybindNew adds a keybind to Fzf.
// NOTE: This is experimental.
//
// <key>:<action>
//
// e.g: "ctrl-o:execute(echo {})".
//
// e.g: "<key>:<action>".
func WithKeybindNew(key, action, desc string) OptFn {
	return func(o *Options) {
		o.header = appendKeyDescToHeader(o.header, key, desc)
		o.keybind = append(o.keybind, fmt.Sprintf("%s:%s", key, action))
	}
}

// WithMultiSelection adds a keybind to select multiple records.
func WithMultiSelection() OptFn {
	opts := []string{"--highlight-line", "--multi"}
	h := appendKeyDescToHeader(make([]string, 0), "ctrl-a", "toggle-all")
	h = appendKeyDescToHeader(h, "tab", "select")

	return func(o *Options) {
		o.args = append(o.args, opts...)
		o.header = append(o.header, h...)
		o.keybind = append(o.keybind, "ctrl-a:toggle-all")
	}
}

// WithPreview adds a preview window and a keybind to toggle it.
func WithPreview() OptFn {
	opts := []string{
		"--preview-window=~4,+{2}+4/3,<80(up)",
		withCommand("--preview=%s {1} --color=always --frame"),
	}

	return func(o *Options) {
		o.args = append(o.args, opts...)
		o.header = appendKeyDescToHeader(o.header, "ctrl-/", "toggle-preview")
		o.keybind = append(o.keybind, "ctrl-/:toggle-preview")
	}
}

// WithPreviewCustomCmd adds preview with a custom command.
func WithPreviewCustomCmd(cmd string) OptFn {
	opts := []string{"--preview=" + cmd}

	return func(o *Options) {
		o.args = append(o.args, opts...)
		o.header = appendKeyDescToHeader(o.header, "ctrl-/", "toggle-preview")
		o.keybind = append(o.keybind, "ctrl-/:toggle-preview")
	}
}

// WithMultilineView adds multiline view and highlights the entire current line
// in fzf.
func WithMultilineView() OptFn {
	opts := []string{
		"--highlight-line", // Highlight the whole current line
		"--read0",          // Read input delimited by ASCII NUL characters instead of newline characters
	}

	return func(o *Options) {
		o.args = append(o.args, opts...)
	}
}

func New[T comparable](opts ...OptFn) *Menu[T] {
	o := defaultOpts()
	for _, fn := range opts {
		fn(&o)
	}

	return &Menu[T]{
		Options: o,
	}
}

func (m *Menu[T]) GetArgs() []string {
	return m.args
}

// setup loads header, keybind and args from Options.
func (m *Menu[T]) setup() {
	loadHeader(m.header, &m.args)
	loadKeybind(m.keybind, &m.args)
}

// Select runs fzf with the given items and returns the selected items.
func (m *Menu[T]) Select(items *[]T, preprocessor func(T) string) ([]T, error) {
	if len(*items) == 0 {
		return nil, ErrFzfNoRecords
	}

	m.setup()

	if preprocessor == nil {
		preprocessor = toString
	}

	inputChan := formatItems(*items, preprocessor)
	outputChan := make(chan string)
	resultChan := make(chan []T)

	go processOutput(*items, preprocessor, outputChan, resultChan)

	// Build fzf.Options
	options, err := fzf.ParseOptions(m.defaults, m.args)
	if err != nil {
		return nil, fmt.Errorf("fzf: %w", err)
	}

	// Set up input and output channels
	options.Input = inputChan
	options.Output = outputChan
	// Run fzf
	code, err := fzf.Run(options)
	if code != 0 {
		exitWithErrCode(code, err)
	}

	close(outputChan)
	result := <-resultChan

	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return result, nil
}
