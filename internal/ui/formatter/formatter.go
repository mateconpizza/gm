package formatter

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mateconpizza/gm/internal/ui"
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

	Parameters Format = "parameters"
)

// Func formats a bookmark for console output.
type Func func(*ui.Console, *bookmark.Bookmark) string

// Formatter defines a bookmark formatting function and optional transform.
type Formatter struct {
	Name   Format
	Render Func
	Menu   MenuConfig
	Hidden bool
}

type MenuConfig struct {
	// Placeholder defines the fzf field expression used to reference
	// the selected item(s) in preview commands and keybindings.
	// Examples: "{1}", "{+1}", "{2}".
	Placeholder string

	// Opts contains additional menu options applied when building
	// the FZF menu. These typically control rendering behavior
	// (e.g., column projection, layout, preview settings).
	Opts []menu.Option
}

var Formatters = map[Format]Formatter{
	Brief: {
		Name:   Brief,
		Render: BriefFunc,
		Menu: MenuConfig{
			Placeholder: "{+2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Flow: {
		Name:   Flow,
		Render: FlowFunc,
		Menu: MenuConfig{
			Placeholder: "{+1}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Oneline: {
		Name:   Oneline,
		Render: OnelineFunc,
		Menu: MenuConfig{
			Placeholder: "{+1}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Multiline: {
		Name:   Multiline,
		Render: MultilineFunc,
		Menu: MenuConfig{
			Placeholder: "{+1}",
			Opts:        []menu.Option{menu.WithMultilineView()},
		},
	},

	bar: {
		Name:   bar,
		Render: BarFunc,
		Menu: MenuConfig{
			Placeholder: "{+2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Frame: {
		Name:   Frame,
		Render: FrameFunc,
		Menu: MenuConfig{
			Placeholder: "{+2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
		Hidden: true,
	},

	Parameters: {
		Name:   Parameters,
		Render: OnelineURLFunc,
		Menu: MenuConfig{
			Placeholder: "{+2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
		Hidden: true,
	},

	Card: {
		Name:   Card,
		Render: CardLiteFunc,
		Menu: MenuConfig{
			Placeholder: "{+1}",
			Opts:        []menu.Option{menu.WithNth("2..")},
		},
	},

	Mini: {
		Name:   Mini,
		Render: MiniFunc,
		Menu: MenuConfig{
			Placeholder: "{+2}",
			Opts:        []menu.Option{menu.WithNth("3..")},
		},
	},

	Minimal: {
		Render: MinimalFunc,
		Menu: MenuConfig{
			Placeholder: "{+1}",
			Opts:        []menu.Option{menu.WithNth("2..")},
		},
	},

	// Compact:   {Format: CompactFunc, Transform: "3..", Preview: "{+2}"},
	// Label:     {Format: LabelFunc, Transform: "3..", Preview: "{+2}"},
	// Lean:      {Format: LeanFunc, Transform: "3..", Preview: "{+2}"},
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

func New(name string) (Formatter, error) {
	if f, ok := Formatters[Format(name)]; ok {
		return f, nil
	}
	return Formatter{}, fmt.Errorf("%w: %q (use: %s)", ErrUnknownFormatter, name, strings.Join(ValidFormats(), "|"))
}

func RegisterFormatter(name string, f Formatter) {
	Formatters[Format(name)] = f
}

func Resolve(output string) (Formatter, error) {
	if output == "" {
		return Formatters[DefFormatter], nil
	}
	fm, ok := Formatters[Format(output)]
	if !ok {
		return Formatter{}, fmt.Errorf("%w: %q", ErrUnknownFormatter, output)
	}
	return fm, nil
}
