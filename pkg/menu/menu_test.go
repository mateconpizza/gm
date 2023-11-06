package menu

import (
	"fmt"
	"testing"

	"gomarks/pkg/errs"
)

func TestMenuCollection(t *testing.T) {
	mc := make(menuCollection)
	rofiMenu := Menu{Command: "rofi", Arguments: []string{"-dmenu"}}
	dmenuMenu := Menu{Command: "dmenu", Arguments: []string{}}

	mc.register(rofiMenu)
	mc.register(dmenuMenu)

	t.Run("Get Registered Menu", func(t *testing.T) {
		menu, err := mc.get("rofi")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if menu.Command != "rofi" {
			t.Errorf("Expected 'rofi' menu, got: %v", menu.Command)
		}
	})

	t.Run("Get Unregistered Menu", func(t *testing.T) {
		menuNonExistent := "nonexistent"
		_, err := mc.get(menuNonExistent)
		expectedError := fmt.Errorf("%w: '%s'", errs.ErrOptionInvalid, menuNonExistent)
		if err == nil || err.Error() != expectedError.Error() {
			t.Errorf("Expected error: %s, got: %v", expectedError, err)
		}
	})
}
