package terminal

import "strings"

// Choice is one option split into the typeable prefix and the rest.
type Choice struct {
	Hot, Tail string // original casing
	IsDefault bool
}

// parseChoices lowercases the options and finds the shortest unique prefix of
// each one. The default is capitalized and placed last.
func parseChoices(opts []string, def string) []Choice {
	lower := make([]string, len(opts))
	for i, o := range opts {
		lower[i] = strings.ToLower(o)
	}

	out := make([]Choice, 0, len(opts))

	var defChoice *Choice

	for i, o := range opts {
		n := uniquePrefixLen(lower[i], lower)
		c := Choice{Hot: o[:n], Tail: o[n:], IsDefault: o == def}

		if c.IsDefault {
			defChoice = &c

			continue
		}

		out = append(out, c)
	}

	if defChoice != nil {
		out = append(out, *defChoice)
	}

	return out
}

// inlineCfg holds the settings for the inline choices formatter. A nil style
// falls back to the Colorizer passed at format time.
type inlineCfg struct {
	left, sep, right string

	hotkey func(string) string                  // hotkey letter(s)
	tail   func(string) string                  // rest of each choice
	delim  func(string) string                  // left, separator and right
	choice func(cz *Colorizer, c Choice) string // replaces per-choice rendering

	defHot func(string) string // formats the default choice's hotkey, before styling
}

// InlineOpt configures ChoicesInlineFmtWith.
type InlineOpt func(*inlineCfg)

// WithInlineDelims sets the left, separator and right strings.
func WithInlineDelims(left, sep, right string) InlineOpt {
	return func(c *inlineCfg) { c.left, c.sep, c.right = left, sep, right }
}

// WithInlineHotkeyStyle overrides how hotkeys are styled.
func WithInlineHotkeyStyle(fn func(string) string) InlineOpt {
	return func(c *inlineCfg) {
		if fn != nil {
			c.hotkey = fn
		}
	}
}

// WithInlineTailStyle overrides how the rest of each choice is styled.
func WithInlineTailStyle(fn func(string) string) InlineOpt {
	return func(c *inlineCfg) {
		if fn != nil {
			c.tail = fn
		}
	}
}

// WithInlineDelimStyle overrides how the left, separator and right strings
// are styled.
func WithInlineDelimStyle(fn func(string) string) InlineOpt {
	return func(c *inlineCfg) {
		if fn != nil {
			c.delim = fn
		}
	}
}

// WithInlineDefault overrides how the default choice's hotkey is rendered
// (for example strings.ToUpper). It receives the lowercased hotkey and runs
// before the hotkey style is applied.
func WithInlineDefault(fn func(hot string) string) InlineOpt {
	return func(c *inlineCfg) {
		if fn != nil {
			c.defHot = fn
		}
	}
}

// WithInlineChoice replaces how each choice is rendered. The styling,
// casing and default options are ignored when it is set; delimiters still apply.
func WithInlineChoice(fn func(cz *Colorizer, c Choice) string) InlineOpt {
	return func(cfg *inlineCfg) {
		if fn != nil {
			cfg.choice = fn
		}
	}
}

// ChoicesInlineFmtWith returns an inline choices formatter configured by opts.
// With no options it uses empty delimiters, the Colorizer's Hotkey and Muted
// styles, and capitalizes the default choice's first letter.
func ChoicesInlineFmtWith(options ...InlineOpt) ChoicesFmt {
	cfg := inlineCfg{defHot: capitalize}
	for _, o := range options {
		o(&cfg)
	}

	return func(cz *Colorizer, opts []string, def string) string {
		delim := cfg.delim
		if delim == nil {
			delim = cz.Muted
		}

		render := cfg.choice
		if render == nil {
			hotkey, tail := cfg.hotkey, cfg.tail
			if hotkey == nil {
				hotkey = cz.Hotkey
			}
			if tail == nil {
				tail = cz.Muted
			}
			render = func(_ *Colorizer, c Choice) string {
				hot, rest := strings.ToLower(c.Hot), strings.ToLower(c.Tail)
				if c.IsDefault && hot != "" {
					hot = cfg.defHot(hot)
				}

				return hotkey(hot) + tail(rest)
			}
		}

		parts := make([]string, 0, len(opts))
		for _, c := range parseChoices(opts, def) {
			parts = append(parts, render(cz, c))
		}

		return delim(cfg.left) + strings.Join(parts, delim(cfg.sep)) + delim(cfg.right)
	}
}

// ChoicesInlineFmt returns an inline choices formatter with the given
// delimiters and the default styles.
func ChoicesInlineFmt(left, separator, right string) ChoicesFmt {
	return ChoicesInlineFmtWith(WithInlineDelims(left, separator, right))
}

// ChoicesInlineBracketedFmt renders the options as [t/b/M]:, highlighting the
// shortest prefix that identifies each one, with the default capitalized and
// last.
func ChoicesInlineBracketedFmt(cz *Colorizer, opts []string, def string) string {
	return ChoicesInlineFmt("[", "/", "]")(cz, opts, def)
}

// ChoicesInlineParenthesisFmt renders the options as (t/b/M):, highlighting
// the shortest prefix that identifies each one, with the default capitalized
// and last.
func ChoicesInlineParenthesisFmt(cz *Colorizer, opts []string, def string) string {
	return ChoicesInlineFmt("(", "/", ")")(cz, opts, def)
}

// ChoicesKeepCaseFmt keeps each option as written: [GPG/JSON/Abort]:
// The default goes last and uses the success color.
var ChoicesKeepCaseFmt = ChoicesInlineFmtWith(
	WithInlineDelims("[", "/", "]:"),
	WithInlineChoice(func(cz *Colorizer, c Choice) string {
		if c.IsDefault {
			return cz.Success(c.Hot + c.Tail)
		}

		return cz.Hotkey(c.Hot) + cz.Muted(c.Tail)
	}),
)

// ChoicesBracketedFmt marks the typeable prefix: (G)PG (J)SON [A]bort:
// The default uses square brackets, the others use parentheses.
var ChoicesBracketedFmt = ChoicesInlineFmtWith(
	WithInlineDelims("", " ", ":"),
	WithInlineChoice(func(cz *Colorizer, c Choice) string {
		open, closing, hot := "(", ")", c.Hot
		if c.IsDefault {
			open, closing, hot = "[", "]", strings.ToUpper(hot)
		}

		return cz.Muted(open) + cz.Hotkey(hot) + cz.Muted(closing+c.Tail)
	}),
)

// ChoicesShortFmt shows only the typeable prefix, yes/no style: [Y/n]:
// The default is uppercase in the success color, the rest lowercase.
var ChoicesShortFmt = ChoicesInlineFmtWith(
	WithInlineDelims("[", "/", "]:"),
	WithInlineChoice(func(cz *Colorizer, c Choice) string {
		if c.IsDefault {
			return cz.Success(strings.ToUpper(c.Hot))
		}

		return cz.Hotkey(strings.ToLower(c.Hot))
	}),
)

// ChoicesPipeFmt separates whole words with pipes and capitalizes the
// default: (Gpg | json | abort):.
var ChoicesPipeFmt = ChoicesInlineFmtWith(
	WithInlineDelims("(", " | ", "):"),
)

// ChoicesMinimalFmt has no delimiters and uppercases the default's hotkey:
// Gpg json abort:.
var ChoicesMinimalFmt = ChoicesInlineFmtWith(
	WithInlineDelims("", " ", ":"),
	WithInlineDefault(strings.ToUpper),
)

// ChoicesMarkedFmt marks the typeable prefix with parentheses and flags the
// default with a trailing asterisk: (g)pg (j)son (a)bort*:.
var ChoicesMarkedFmt = ChoicesInlineFmtWith(
	WithInlineDelims("", " ", ":"),
	WithInlineChoice(func(cz *Colorizer, c Choice) string {
		if c.IsDefault {
			return cz.Muted("(") + cz.Success(c.Hot) + cz.Muted(")") +
				cz.Success(c.Tail) + cz.Muted("*")
		}

		return cz.Muted("(") + cz.Hotkey(c.Hot) + cz.Muted(")"+c.Tail)
	}),
)

// ChoicesArrowFmt renders options in a step-by-step navigation style: ➔
// Y/n/a:.
var ChoicesArrowFmt = ChoicesInlineFmtWith(
	WithInlineDelims("➔ ", "/", ":"),
	WithInlineDefault(strings.ToUpper),
)

// ChoicesBadgeFmt renders options inside badge-like brackets with bold tags:
// <[y]es> / <[n]o>:.
var ChoicesBadgeFmt = ChoicesInlineFmtWith(
	WithInlineDelims("<", " / ", ">:"),
	WithInlineChoice(func(cz *Colorizer, c Choice) string {
		if c.IsDefault {
			return cz.Success("[" + strings.ToUpper(c.Hot) + "]" + c.Tail)
		}
		return cz.Muted("[") + cz.Hotkey(c.Hot) + cz.Muted("]"+c.Tail)
	}),
)
