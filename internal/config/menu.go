package config

import (
	"fmt"
	"strings"

	"github.com/mateconpizza/gm/internal/ui/menu"
)

func fmtKeybindCmd(s string) string {
	return fmt.Sprintf("%s --name=%s records %s", cfg.Cmd, cfg.DBName, s)
}

// MenuKeybindEdit keybind to edit the selected record.
func MenuKeybindEdit(args ...string) *menu.Keymap {
	cmd := "--edit "
	if len(args) > 0 {
		cmd += strings.Join(args, " ") + " "
	}
	cmd += "{+1}"

	return cfg.Menu.BuiltinKeymaps.Edit.WithAction(fmtKeybindCmd(cmd))
}

// MenuKeybindEditNotes keybind to edit the selected record.
func MenuKeybindEditNotes() *menu.Keymap {
	return cfg.Menu.BuiltinKeymaps.EditNotes.WithAction(fmtKeybindCmd("--edit --notes {+1}"))
}

// MenuKeybindOpen keybind to open the selected record in the default browser.
func MenuKeybindOpen() *menu.Keymap {
	return cfg.Menu.BuiltinKeymaps.Open.WithAction(fmtKeybindCmd("--open {+1}"))
}

// MenuKeybindQR keybind to show the QR code of the selected record.
func MenuKeybindQR() *menu.Keymap {
	return cfg.Menu.BuiltinKeymaps.QR.WithAction(fmtKeybindCmd("--qr {+1}"))
}

// MenuKeybindOpenQR keybind to open the QR code of the selected record in the
// default image viewer.
func MenuKeybindOpenQR() *menu.Keymap {
	return cfg.Menu.BuiltinKeymaps.OpenQR.WithAction(fmtKeybindCmd("--qr --open {+1}"))
}

// MenuKeybindYank keybind to copy the selected record to the system clipboard.
func MenuKeybindYank() *menu.Keymap {
	return cfg.Menu.BuiltinKeymaps.Yank.WithAction(fmtKeybindCmd("--copy {+1}"))
}
