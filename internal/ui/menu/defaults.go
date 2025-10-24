package menu

const (
	unicodePathBigSegment = "\u25B6" // ▶
	unicodeMiddleDot      = "\u00b7" // ·
	DefaultPrompt         = unicodePathBigSegment + " "
	DefaultHeaderSep      = " " + unicodeMiddleDot + " "
)

func NewDefaultConfig() *Config {
	return &Config{
		Defaults: true,
		Prompt:   DefaultPrompt,
		Preview:  true,
		Header: Header{
			Enabled: true,
			Sep:     DefaultHeaderSep,
		},
		BuiltinKeymaps: &Keymaps{
			Edit:      &Keymap{Bind: "ctrl-e", Desc: "edit", Enabled: true, Hidden: false},
			EditNotes: &Keymap{Bind: "ctrl-w", Desc: "edit notes", Enabled: true, Hidden: false},
			Open:      &Keymap{Bind: "ctrl-o", Desc: "open", Enabled: true, Hidden: false},
			OpenQR:    &Keymap{Bind: "ctrl-l", Desc: "openQR", Enabled: true, Hidden: false},
			Preview:   &Keymap{Bind: "ctrl-/", Desc: "toggle-preview", Enabled: true, Hidden: false},
			QR:        &Keymap{Bind: "ctrl-k", Desc: "QRcode", Enabled: true, Hidden: false},
			ToggleAll: &Keymap{Bind: "ctrl-a", Desc: "toggle-all", Enabled: true, Hidden: false},
			Yank:      &Keymap{Bind: "ctrl-y", Desc: "yank", Enabled: true, Hidden: false},
		},
		Arguments: Args{
			"--ansi",                            // Enable processing of ANSI color codes
			"--reverse",                         // A synonym for --layout=reverse
			"--sync",                            // Synchronous search for multi-staged filtering
			"--info=inline-right",               // Determines the display style of the finder info.
			"--tac",                             // Reverse the order of the input
			"--layout=default",                  // Choose the layout (default: default)
			"--color=prompt:bold",               // Prompt style
			"--color=header:italic:bright-blue", // Header style
			"--height=100%",                     // Set the height of the menu
			"--no-scrollbar",                    // Remove scrollbar
			"--border-label= GoMarks ",          // Label to print on the horizontal border line
			"--border",                          // Border around the window
		},
	}
}
