package formatter

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

var (
	ErrUnknownFormatter = errors.New("unknown formatter")
	ErrInvalidFormat    = errors.New("invalid format")
)

type Format string

const (
	DefFormatter Format = "oneline"

	Brief     Format = "brief"
	Card      Format = "card"
	Compact   Format = "compact"
	Flow      Format = "flow"
	Label     Format = "label"
	Lean      Format = "lean"
	Mini      Format = "mini"
	Minimal   Format = "minimal"
	Multiline Format = "multiline"
	Oneline   Format = "oneline"
	bar       Format = "bar"
	Frame     Format = "frame"

	ArchiveURL     Format = "archiveURL"
	Parameters     Format = "parameters"
	HTTPStatusCode Format = "statusCode"
	Notes          Format = "notes"
)

func (f Format) String() string { return string(f) }

// Func formats a bookmark for console output.
type Func func(Console, *bookmark.Bookmark) string

// Formatter defines a bookmark formatting function and optional transform.
type Formatter struct {
	Name   Format
	Render Func
	Menu   MenuConfig
	Hidden bool
}

type Placeholder string

func (p Placeholder) String() string { return string(p) }

// Single returns the placeholder for the current item.
func (p Placeholder) Single() string { return strings.ReplaceAll(string(p), "+", "") }

// Multi returns the placeholder modified for fzf multi-selection.
func (p Placeholder) Multi() string {
	s := string(p)
	if strings.Contains(s, "{+") {
		return s
	}
	return strings.Replace(s, "{", "{+", 1)
}

type MenuConfig struct {
	// Placeholder defines the fzf field expression used to reference
	// the selected item(s) in preview commands and keybindings.
	// Examples: "{1}", "{+1}", "{2}".
	placeholder Placeholder

	// Opts contains additional menu options applied when building
	// the FZF menu. These typically control rendering behavior
	// (e.g., column projection, layout, preview settings).
	Opts []menu.Option
}

func (m MenuConfig) Placeholder() Placeholder { return m.placeholder }

var Formatters = map[Format]Formatter{
	Brief: {
		Name:   Brief,
		Render: BriefFunc,
		Menu: MenuConfig{
			placeholder: "{2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Flow: {
		Name:   Flow,
		Render: FlowFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
		},
	},

	Oneline: {
		Name:   Oneline,
		Render: OnelineFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Multiline: {
		Name:   Multiline,
		Render: MultilineFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithMultilineView()},
		},
	},

	bar: {
		Name:   bar,
		Render: BarFunc,
		Menu: MenuConfig{
			placeholder: "{2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Frame: {
		Name:   Frame,
		Render: FrameFunc,
		Menu: MenuConfig{
			placeholder: "{2}",
			Opts:        []menu.Option{menu.WithNth("3.."), menu.WithMultilineView()},
		},
		Hidden: true,
	},

	Parameters: {
		Name:   Parameters,
		Render: OnelineURLFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
		Hidden: true,
	},

	Card: {
		Name:   Card,
		Render: CardLiteFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("2.."), menu.WithMultilineView()},
		},
	},

	Mini: {
		Name:   Mini,
		Render: MiniFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("2..")},
		},
	},

	Minimal: {
		Render: MinimalFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("2..")},
		},
	},

	ArchiveURL: {
		Render: ArchiveURLFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("2..")},
		},
		Hidden: true,
	},

	HTTPStatusCode: {
		Render: StatusCodeFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithNth("2..")},
		},
		Hidden: true,
	},

	Notes: {
		Render: NotesFunc,
		Menu: MenuConfig{
			placeholder: "{1}",
			Opts:        []menu.Option{menu.WithMultilineView()},
		},
		Hidden: true,
	},
}

var ValidFormats = func() []string {
	keys := make([]string, 0, len(Formatters))
	for k, v := range Formatters {
		if !v.Hidden {
			keys = append(keys, string(k))
		}
	}
	slices.Sort(keys)
	return keys
}

func New(name Format) (Formatter, error) {
	if name == "" {
		return Formatters[DefFormatter], nil
	}

	if f, ok := Formatters[name]; ok {
		return f, nil
	}
	return Formatter{}, fmt.Errorf("%w: %q (use: %s)", ErrUnknownFormatter, name, strings.Join(ValidFormats(), ", "))
}

func RegisterFormatter(name string, f Formatter) {
	Formatters[Format(name)] = f
}

// Default returns the default formatter: oneline.
func Default() Formatter { return Formatters[DefFormatter] }
