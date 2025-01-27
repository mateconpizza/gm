package menu

import (
	"errors"
	"fmt"
	"log"

	fzf "github.com/junegunn/fzf/src"
)

var addColor *bool

var (
	ErrFzfExitError     = errors.New("fzf: exit error")
	ErrFzfInterrupted   = errors.New("fzf: returned exit code 130")
	ErrFzfNoRecords     = errors.New("fzf: no records provided")
	ErrFzfReturnCode    = errors.New("fzf: returned a non-zero code")
	ErrFzfActionAborted = errors.New("fzf: action aborted: code 130")
)

var fzfDefaults = []string{
	"--ansi",                // Enable processing of ANSI color codes
	"--cycle",               // Enable cyclic scroll
	"--reverse",             // A synonym for --layout=reverse
	"--sync",                // Synchronous search for multi-staged filtering
	"--info=inline-right",   // Determines the display style of the finder info.
	"--tac",                 // Reverse the order of the input
	"--layout=default",      // Choose the layout (default: default)
	"--color=header:italic", // Header style
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

// AddOpts adds options to the menu.
func (m *Menu[T]) AddOpts(opts ...OptFn) {
	for _, fn := range opts {
		fn(&m.Options)
	}
}

// defaultOpts returns the default options.
func defaultOpts() Options {
	return Options{
		args:     append(fzfDefaults, "--prompt="+menuConfig.Prompt),
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
	edit := menuConfig.Keymaps.Edit
	if !edit.Enabled {
		return func(o *Options) {}
	}

	return func(o *Options) {
		if !edit.Hidden {
			o.header = appendKeytoHeader(o.header, edit.Bind, edit.Description)
		}
		o.keybind = append(o.keybind, withCommand(edit.Bind+":execute(%s --edit {1})"))
	}
}

// WithKeybindOpen adds a keybind to open the selected record in default
// browser.
func WithKeybindOpen() OptFn {
	open := menuConfig.Keymaps.Open
	if !open.Enabled {
		return func(o *Options) {}
	}

	return func(o *Options) {
		if !open.Hidden {
			o.header = appendKeytoHeader(o.header, open.Bind, open.Description)
		}
		o.keybind = append(o.keybind, withCommand(open.Bind+":execute(%s --open {1})"))
	}
}

// WithKeybindQR adds a keybinding to generate a QR code.
func WithKeybindQR() OptFn {
	qr := menuConfig.Keymaps.QR
	if !qr.Enabled {
		return func(o *Options) {}
	}

	return func(o *Options) {
		if !qr.Hidden {
			o.header = appendKeytoHeader(o.header, qr.Bind, qr.Description)
		}
		o.keybind = append(o.keybind, withCommand(qr.Bind+":execute(%s --qr {1})"))
	}
}

// WithKeybindOpenQR adds a keybinding to generate and open a QR code in the
// default image viewer.
func WithKeybindOpenQR() OptFn {
	qr := menuConfig.Keymaps.OpenQR
	if !qr.Enabled {
		return func(o *Options) {}
	}

	return func(o *Options) {
		if !qr.Hidden {
			o.header = appendKeytoHeader(o.header, qr.Bind, qr.Description)
		}
		o.keybind = append(o.keybind, withCommand(qr.Bind+":execute(%s --qr --open {1})"))
	}
}

// WithDefaultKeybinds adds default keybinds to Fzf.
//
// ctrl-y:copy-to-clipboard.
func WithDefaultKeybinds() OptFn {
	yank := menuConfig.Keymaps.Yank
	if !yank.Enabled {
		return func(o *Options) {}
	}

	return func(o *Options) {
		if !yank.Hidden {
			o.header = appendKeytoHeader(o.header, yank.Bind, yank.Description)
		}
		o.keybind = append(o.keybind, withCommand(yank.Bind+":execute(%s --copy {1})"))
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
		o.header = appendKeytoHeader(o.header, key, desc)
		o.keybind = append(o.keybind, fmt.Sprintf("%s:%s", key, action))
	}
}

// WithMultiSelection adds a keybind to select multiple records.
func WithMultiSelection() OptFn {
	opts := []string{"--highlight-line", "--multi"}
	if !menuConfig.Keymaps.ToggleAll.Enabled {
		return func(o *Options) {
			o.args = append(o.args, opts...)
		}
	}

	h := appendKeytoHeader(make([]string, 0), "ctrl-a", "toggle-all")

	return func(o *Options) {
		o.args = append(o.args, opts...)
		if !menuConfig.Keymaps.ToggleAll.Hidden {
			o.header = append(o.header, h...)
		}
		o.keybind = append(o.keybind, "ctrl-a:toggle-all")
	}
}

func WithColor(b *bool) {
	addColor = b
}

// WithPreview adds a preview window and a keybind to toggle it.
func WithPreview() OptFn {
	// TODO: sometimes is better to show the preview window, e.g: selecting
	// databases
	if !menuConfig.Preview {
		return func(o *Options) {}
	}

	preview := menuConfig.Keymaps.Preview
	if !preview.Enabled {
		return func(o *Options) {}
	}

	withColor := "never"
	if *addColor {
		withColor = "always"
	}

	opts := []string{
		"--preview-window=~4,+{2}+4/3,<80(up)",
		withCommand("--preview=%s {1} --color=" + withColor),
	}

	return func(o *Options) {
		o.args = append(o.args, opts...)
		if !preview.Hidden {
			o.header = appendKeytoHeader(o.header, preview.Bind, "toggle-preview")
		}
		o.keybind = append(o.keybind, preview.Bind+":toggle-preview")
	}
}

// WithPreviewCustomCmd adds preview with a custom command.
func WithPreviewCustomCmd(cmd string) OptFn {
	opts := []string{"--preview=" + cmd}

	return func(o *Options) {
		o.args = append(o.args, opts...)
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
		o.args = append(o.args, opts...)
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
		o.args = append(o.args, "--prompt="+prompt)
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

func (m *Menu[T]) GetArgs() []string {
	return m.args
}

// setup loads header, keybind and args from Options.
func (m *Menu[T]) setup() error {
	loadHeader(m.header, &m.args)
	return loadKeybind(m.keybind, &m.args)
}

// Select runs Fzf with the given items and returns the selected item/s.
func (m *Menu[T]) Select(items *[]T, preprocessor func(*T) string) ([]T, error) {
	if len(*items) == 0 {
		return nil, ErrFzfNoRecords
	}

	if err := m.setup(); err != nil {
		return nil, err
	}

	if preprocessor == nil {
		log.Print("preprocessor is nil")
		preprocessor = toString
	}

	log.Printf("menu args: %v", m.args)

	// channels
	inputChan := formatItems(*items, preprocessor)
	outputChan := make(chan string)
	resultChan := make(chan []T)

	go processOutput(*items, preprocessor, outputChan, resultChan)

	// Build Fzf.Options
	options, err := fzf.ParseOptions(m.defaults, m.args)
	if err != nil {
		return nil, fmt.Errorf("fzf: %w", err)
	}

	// Set up input and output channels
	options.Input = inputChan
	options.Output = outputChan

	// Run Fzf
	retcode, err := fzf.Run(options)
	if retcode != 0 {
		return nil, handleFzfErr(retcode)
	}

	close(outputChan)
	result := <-resultChan

	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return result, nil
}
