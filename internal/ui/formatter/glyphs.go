package formatter

import (
	"errors"
	"reflect"
)

var ErrGlyphModeInvalid = errors.New("glyphs mode invalid")

type GlyphMode string

const (
	GlyphModeASCII         GlyphMode = "ascii"
	GlyphModeUnicode       GlyphMode = "unicode"
	GlyphModeCustom        GlyphMode = "custom"
	GlyphModeCustomUnicode GlyphMode = "custom-unicode"

	GlyphDefault GlyphMode = GlyphModeASCII
)

func (m GlyphMode) Valid() error {
	switch m {
	case GlyphModeASCII, GlyphModeUnicode, GlyphModeCustom, GlyphModeCustomUnicode:
		return nil
	default:
		return ErrGlyphModeInvalid
	}
}

type Glyphs struct {
	// ui
	Ellipsis string `json:"ellipsis,omitempty" yaml:"ellipsis,omitempty"`
	Sep      string `json:"sep,omitempty"      yaml:"sep,omitempty"`
	Pointer  string `json:"pointer,omitempty"  yaml:"pointer,omitempty"`
	Git      string `json:"git,omitempty"      yaml:"git,omitempty"`
	JSON     string `json:"json,omitempty"     yaml:"json,omitempty"`
	GPG      string `json:"gpg,omitempty"      yaml:"gpg,omitempty"`

	// logs
	Warning  string `json:"warning,omitempty"  yaml:"warning,omitempty"`
	Question string `json:"question,omitempty" yaml:"question,omitempty"`
	Info     string `json:"info,omitempty"     yaml:"info,omitempty"`
	Error    string `json:"error,omitempty"    yaml:"error,omitempty"`
	Success  string `json:"success,omitempty"  yaml:"success,omitempty"`

	// bookmark
	Favorite string `json:"favorite,omitempty" yaml:"favorite,omitempty"`
	Notes    string `json:"notes,omitempty"    yaml:"notes,omitempty"`
	Archive  string `json:"archive,omitempty"  yaml:"archive,omitempty"`
	Broken   string `json:"broken,omitempty"   yaml:"broken,omitempty"`
	Inactive string `json:"inactive,omitempty" yaml:"inactive,omitempty"`

	// actions
	Add    string `json:"add,omitempty"    yaml:"add,omitempty"`
	Update string `json:"update,omitempty" yaml:"update,omitempty"`
	Del    string `json:"del,omitempty"    yaml:"del,omitempty"`

	// misc
	Square           string `json:"black_square,omitempty"       yaml:"black_square,omitempty"`
	HeavyVertical    string `json:"heavy_vertical,omitempty"     yaml:"heavy_vertical,omitempty"`
	RightDoubleAngle string `json:"right_double_angle,omitempty" yaml:"right_double_angle,omitempty"`
	SeparatorLeft    string `json:"separator_left,omitempty"     yaml:"separator_left,omitempty"`
	SeparatorRight   string `json:"separator_right,omitempty"    yaml:"separator_right,omitempty"`
}

func NewGlyphs(mode GlyphMode, custom *Glyphs) *Glyphs {
	switch mode {
	case GlyphModeUnicode:
		return Unicode
	case GlyphModeCustom:
		return merge(custom, ASCII)
	case GlyphModeCustomUnicode:
		return merge(custom, Unicode)
	case GlyphModeASCII:
		return ASCII
	default:
		return ASCII
	}
}

var (
	Unicode = &Glyphs{
		// ui
		Ellipsis: "…",
		Sep:      "·",
		Pointer:  "›",
		Git:      "",
		JSON:     "",
		GPG:      "󰌾",

		// log
		Warning:  "",
		Question: "",
		Info:     "",
		Error:    "",
		Success:  "󰸞",

		// bookmark
		Favorite: " ",
		Notes:    " ",
		Archive:  "󰚰 ",
		Broken:   "󰌸 ",
		Inactive: "󰧎 ",

		// actions
		Add:    "",
		Update: "󰓦",
		Del:    "󰆴",

		// misc
		Square:           "▪",
		HeavyVertical:    "┃",
		RightDoubleAngle: "»",
		SeparatorLeft:    "",
		SeparatorRight:   "",
	}

	ASCII = &Glyphs{
		// ui
		Ellipsis: "...",
		Sep:      "-",
		Pointer:  ">",
		Git:      "git",
		JSON:     "json",
		GPG:      "gpg",

		// log
		Warning:  "!",
		Question: "?",
		Info:     "i",
		Error:    "x",
		Success:  "+",

		// bookmark
		Favorite: "*",
		Notes:    "~",
		Archive:  "@",
		Broken:   "?",
		Inactive: "x",

		// actions
		Add:    "▪",
		Update: "▪",
		Del:    "▪",

		// misc
		Square:           "▪",
		HeavyVertical:    "|",
		RightDoubleAngle: ">>",
		SeparatorLeft:    "[",
		SeparatorRight:   "]",
	}
)

// merge returns a copy of fallback with every non-empty field in custom
// overlaid on top, so a partial user config only overrides what it sets.
func merge(custom, fallback *Glyphs) *Glyphs {
	if custom == nil {
		return fallback
	}

	merged := *fallback // copy, so fallback itself is never mutated

	cv := reflect.ValueOf(custom).Elem()
	mv := reflect.ValueOf(&merged).Elem()

	for i := range cv.NumField() {
		if s := cv.Field(i).String(); s != "" {
			mv.Field(i).SetString(s)
		}
	}

	return &merged
}
