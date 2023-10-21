package menu_test

import (
	m "gomarks/pkg/menu"
	"testing"
)

func TestMenuCollection(t *testing.T) {
	mc := make(m.MenuCollection)
	rofiMenu := m.Menu{Command: "rofi", Arguments: []string{"-dmenu"}}
	dmenuMenu := m.Menu{Command: "dmenu", Arguments: []string{}}

	mc.Register(rofiMenu)
	mc.Register(dmenuMenu)

	t.Run("Get Registered Menu", func(t *testing.T) {
		menu, err := mc.Get("rofi")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if menu.Command != "rofi" {
			t.Errorf("Expected 'rofi' menu, got: %v", menu.Command)
		}
	})

	t.Run("Get Unregistered Menu", func(t *testing.T) {
		_, err := mc.Get("nonexistent")
		expectedError := "menu 'nonexistent' not found"
		if err == nil || err.Error() != expectedError {
			t.Errorf("Expected error: %s, got: %v", expectedError, err)
		}
	})
}
