package menu

import "testing"

func TestNewArgsBuilder(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()

	if builder == nil {
		t.Fatal("newArgsBuilder() returned nil")
	}

	// Verify all fields are initialized with expected values
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"ansi", builder.ansi, "--ansi"},
		{"bind", builder.bind, "--bind"},
		{"border", builder.border, "--border"},
		{"borderLabel", builder.borderLabel, "--border-label"},
		{"color", builder.color, "--color"},
		{"header", builder.header, "--header"},
		{"height", builder.height, "--height"},
		{"highlightLine", builder.highlightLine, "--highlight-line"},
		{"info", builder.info, "--info"},
		{"layout", builder.layout, "--layout"},
		{"multi", builder.multi, "--multi"},
		{"noColor", builder.noColor, "--no-color"},
		{"noScrollbar", builder.noScrollbar, "--no-scrollbar"},
		{"pointer", builder.pointer, "--pointer"},
		{"preview", builder.preview, "--preview"},
		{"previewWindow", builder.previewWindow, "--preview-window"},
		{"prompt", builder.prompt, "--prompt"},
		{"read0", builder.read0, "--read0"},
		{"sync", builder.sync, "--sync"},
		{"tac", builder.tac, "--tac"},
		{"footer", builder.footer, "--footer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.expected {
				t.Errorf("field %s = %q, expected %q", tt.name, tt.got, tt.expected)
			}
		})
	}

	if builder.list != nil {
		t.Errorf("list should be nil initially, got %v", builder.list)
	}
}

func TestArgsBuilder_Add(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()

	// Test adding single argument
	result := builder.add("--test")
	if result != builder {
		t.Error("add() should return the same builder instance for chaining")
	}
	if len(builder.list) != 1 || builder.list[0] != "--test" {
		t.Errorf("add() = %v, expected [--test]", builder.list)
	}

	// Test adding multiple arguments
	builder.add("--arg1", "--arg2", "--arg3")
	expected := Args{"--test", "--arg1", "--arg2", "--arg3"}
	if len(builder.list) != len(expected) {
		t.Fatalf("list length = %d, expected %d", len(builder.list), len(expected))
	}
	for i, v := range expected {
		if builder.list[i] != v {
			t.Errorf("list[%d] = %q, expected %q", i, builder.list[i], v)
		}
	}

	// Test adding empty slice
	builder.add()
	if len(builder.list) != len(expected) {
		t.Errorf("add() with no args should not modify list, got %v", builder.list)
	}
}

func TestArgsBuilder_WithNoColor(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	result := builder.withNoColor()

	if result != builder {
		t.Error("withNoColor() should return the same builder instance")
	}

	if len(builder.list) != 1 {
		t.Fatalf("list length = %d, expected 1", len(builder.list))
	}

	if builder.list[0] != "--no-color" {
		t.Errorf("withNoColor() added %q, expected --no-color", builder.list[0])
	}
}

func TestArgsBuilder_WithAnsi(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	result := builder.withAnsi()

	if result != builder {
		t.Error("withAnsi() should return the same builder instance")
	}

	if len(builder.list) != 1 {
		t.Fatalf("list length = %d, expected 1", len(builder.list))
	}

	if builder.list[0] != "--ansi" {
		t.Errorf("withAnsi() added %q, expected --ansi", builder.list[0])
	}
}

func TestArgsBuilder_WithTac(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	result := builder.withTac()

	if result != builder {
		t.Error("withTac() should return the same builder instance")
	}

	if len(builder.list) != 1 {
		t.Fatalf("list length = %d, expected 1", len(builder.list))
	}

	if builder.list[0] != "--tac" {
		t.Errorf("withTac() added %q, expected --tac", builder.list[0])
	}
}

func TestArgsBuilder_WithSync(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	result := builder.withSync()

	if result != builder {
		t.Error("withSync() should return the same builder instance")
	}

	if len(builder.list) != 1 {
		t.Fatalf("list length = %d, expected 1", len(builder.list))
	}

	if builder.list[0] != "--sync" {
		t.Errorf("withSync() added %q, expected --sync", builder.list[0])
	}
}

func TestArgsBuilder_WithNoScrollbar(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	result := builder.withNoScrollbar()

	if result != builder {
		t.Error("withNoScrollbar() should return the same builder instance")
	}

	if len(builder.list) != 1 {
		t.Fatalf("list length = %d, expected 1", len(builder.list))
	}

	if builder.list[0] != "--no-scrollbar" {
		t.Errorf("withNoScrollbar() added %q, expected --no-scrollbar", builder.list[0])
	}
}

func TestArgsBuilder_WithLayout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"reverse layout", "reverse", "--layout=reverse"},
		{"default layout", "default", "--layout=default"},
		{"reverse-list", "reverse-list", "--layout=reverse-list"},
		{"empty string", "", "--layout="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withLayout(tt.input)

			if result != builder {
				t.Error("withLayout() should return the same builder instance")
			}

			if len(builder.list) != 1 {
				t.Fatalf("list length = %d, expected 1", len(builder.list))
			}

			if builder.list[0] != tt.expected {
				t.Errorf("withLayout(%q) = %q, expected %q", tt.input, builder.list[0], tt.expected)
			}
		})
	}
}

func TestArgsBuilder_WithPointer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple pointer", "▶", "--pointer=▶"},
		{"arrow pointer", "→", "--pointer=→"},
		{"empty string", "", "--pointer="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withPointer(tt.input)

			if result != builder {
				t.Error("withPointer() should return the same builder instance")
			}

			if len(builder.list) != 1 {
				t.Fatalf("list length = %d, expected 1", len(builder.list))
			}

			if builder.list[0] != tt.expected {
				t.Errorf("withPointer(%q) = %q, expected %q", tt.input, builder.list[0], tt.expected)
			}
		})
	}
}

func TestArgsBuilder_WithPreview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple command", "cat {}", "--preview=cat {}"},
		{"complex command", "bat --color=always {}", "--preview=bat --color=always {}"},
		{"empty string", "", "--preview="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withPreview(tt.input)

			if result != builder {
				t.Error("withPreview() should return the same builder instance")
			}

			if len(builder.list) != 1 {
				t.Fatalf("list length = %d, expected 1", len(builder.list))
			}

			if builder.list[0] != tt.expected {
				t.Errorf("withPreview(%q) = %q, expected %q", tt.input, builder.list[0], tt.expected)
			}
		})
	}
}

func TestArgsBuilder_WithPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple prompt", "Search: ", "--prompt=Search: "},
		{"unicode prompt", "🔍 ", "--prompt=🔍 "},
		{"empty string", "", "--prompt="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withPrompt(tt.input)

			if result != builder {
				t.Error("withPrompt() should return the same builder instance")
			}

			if len(builder.list) != 1 {
				t.Fatalf("list length = %d, expected 1", len(builder.list))
			}

			if builder.list[0] != tt.expected {
				t.Errorf("withPrompt(%q) = %q, expected %q", tt.input, builder.list[0], tt.expected)
			}
		})
	}
}

func TestArgsBuilder_WithInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"inline-right", "inline-right", "--info=inline-right"},
		{"inline", "inline", "--info=inline"},
		{"hidden", "hidden", "--info=hidden"},
		{"empty string", "", "--info="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withInfo(tt.input)

			if result != builder {
				t.Error("withInfo() should return the same builder instance")
			}

			if len(builder.list) != 1 {
				t.Fatalf("list length = %d, expected 1", len(builder.list))
			}

			if builder.list[0] != tt.expected {
				t.Errorf("withInfo(%q) = %q, expected %q", tt.input, builder.list[0], tt.expected)
			}
		})
	}
}

func TestArgsBuilder_WithHeight(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"percentage", "50%", "--height=50%"},
		{"fixed lines", "20", "--height=20"},
		{"full height", "100%", "--height=100%"},
		{"empty string", "", "--height="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withHeight(tt.input)

			if result != builder {
				t.Error("withHeight() should return the same builder instance")
			}

			if len(builder.list) != 1 {
				t.Fatalf("list length = %d, expected 1", len(builder.list))
			}

			if builder.list[0] != tt.expected {
				t.Errorf("withHeight(%q) = %q, expected %q", tt.input, builder.list[0], tt.expected)
			}
		})
	}
}

func TestArgsBuilder_WithBorderLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected Args
	}{
		{"simple label", "Menu", Args{"--border", "--border-label=Menu"}},
		{"label with spaces", "My Menu", Args{"--border", "--border-label=My Menu"}},
		{"empty string", "", Args{"--border", "--border-label="}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := newArgsBuilder()
			result := builder.withBorderLabel(tt.input)

			if result != builder {
				t.Error("withBorderLabel() should return the same builder instance")
			}

			if len(builder.list) != 2 {
				t.Fatalf("list length = %d, expected 2", len(builder.list))
			}

			for i, expected := range tt.expected {
				if builder.list[i] != expected {
					t.Errorf("list[%d] = %q, expected %q", i, builder.list[i], expected)
				}
			}
		})
	}
}

func TestArgsBuilder_Build(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	builder.add("--test1", "--test2")

	result := builder.build()

	if len(result) != 2 {
		t.Fatalf("build() returned %d args, expected 2", len(result))
	}

	expected := Args{"--test1", "--test2"}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("build()[%d] = %q, expected %q", i, result[i], v)
		}
	}

	// Verify it returns the actual list, not a copy
	if &builder.list[0] != &result[0] {
		t.Error("build() should return the same slice, not a copy")
	}
}

func TestArgsBuilder_Chaining(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()

	result := builder.
		withNoColor().
		withAnsi().
		withTac().
		withSync().
		withNoScrollbar().
		withLayout("reverse").
		withPointer("→").
		withPrompt("Select: ").
		withPreview("cat {}").
		withInfo("inline").
		withHeight("50%").
		withColor("prompt", "bold", "blue").
		withColor("header", "italic", "green").
		withBorderLabel("Files").
		build()

	expected := Args{
		"--no-color",
		"--ansi",
		"--tac",
		"--sync",
		"--no-scrollbar",
		"--layout=reverse",
		"--pointer=→",
		"--prompt=Select: ",
		"--preview=cat {}",
		"--info=inline",
		"--height=50%",
		"--color=prompt:bold:blue",
		"--color=header:italic:green",
		"--border",
		"--border-label=Files",
	}

	if len(result) != len(expected) {
		t.Fatalf("chained build() returned %d args, expected %d", len(result), len(expected))
	}

	for i, v := range expected {
		if result[i] != v {
			t.Errorf("result[%d] = %q, expected %q", i, result[i], v)
		}
	}
}

func TestArgsBuilder_EmptyBuild(t *testing.T) {
	t.Parallel()
	builder := newArgsBuilder()
	result := builder.build()

	if result != nil {
		t.Errorf("build() on empty builder = %v, expected nil", result)
	}
}
