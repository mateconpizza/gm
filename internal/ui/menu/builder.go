package menu

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	shellwords "github.com/junegunn/go-shellwords"
)

var ErrInvalidHeaderArg = errors.New("invalid header argument")

// buildArgs loads header, prompt, keybind and args from Options.
func (m *Menu[T]) buildArgs() error {
	if err := m.buildPreviewArgs(); err != nil {
		return err
	}

	if err := m.buildHeaderArgs(); err != nil {
		return err
	}

	if err := m.buildPromptArgs(); err != nil {
		return err
	}

	return m.buildKeybindArgs()
}

// buildHeaderStrings returns the formatted header strings from enabled keymaps.
func (m *Menu[T]) buildHeaderStrings() []string {
	// If we have explicitly set a single header (not appending), use only that
	if len(m.header) == 1 && m.customHeaderOnly {
		return m.header
	}

	if !m.cfg.Header.Enabled {
		return m.header
	}

	headers := make([]string, 0, len(m.header))
	headers = append(headers, m.header...)

	for _, k := range m.keymaps.list() {
		if !k.Enabled || k.Hidden {
			continue
		}

		headers = append(headers, fmt.Sprintf("%s:%s", k.Bind, k.Desc))
	}

	return headers
}

// formatHeaderArg builds the complete `--header="..."` argument string.
func (m *Menu[T]) formatHeaderArgs(headers []string) (string, error) {
	if len(headers) == 0 {
		slog.Debug("menu: skipping header, empty")
		return "", nil
	}

	s := fmt.Sprintf("%s=%q", "--header", strings.Join(headers, m.cfg.Header.Sep))
	args, err := shellwords.Parse(s)
	if err != nil {
		return "", err
	}

	// shellwords.Parse returns a slice; we expect one element here.
	if len(args) == 0 {
		return "", fmt.Errorf("%w parsed from %q", ErrInvalidHeaderArg, s)
	}

	return args[0], nil
}

// buildHeader builds and appends the header argument.
func (m *Menu[T]) buildHeaderArgs() error {
	headers := m.buildHeaderStrings()
	headerArg, err := m.formatHeaderArgs(headers)
	if err != nil {
		return err
	}
	if headerArg == "" {
		return nil
	}

	m.arguments = append(m.arguments, headerArg)

	return nil
}

// buildKeybindString builds the keybind string for FZF.
func (m *Menu[T]) buildKeybindString() string {
	n := m.keymaps.len()
	if n == 0 {
		return ""
	}

	keybinds := make([]string, 0, n)
	for _, k := range m.keymaps.list() {
		if k.Action == "" {
			slog.Warn("build keybind ignore", "bind", k.Bind, "action", k.Action)
			continue
		}

		if k.Enabled {
			keybinds = append(keybinds, fmt.Sprintf("%s:%s", k.Bind, k.Action))
		}
	}

	return strings.Join(keybinds, ",")
}

// buildKeybindArgs appends keybinding arguments to the menu.
func (m *Menu[T]) buildKeybindArgs() error {
	keybindStr := m.buildKeybindString()
	if keybindStr == "" {
		return nil
	}

	bindArg := fmt.Sprintf("--bind=%q", keybindStr)
	binds, err := shellwords.Parse(bindArg)
	if err != nil {
		return fmt.Errorf("parse keybinds %q: %w", keybindStr, err)
	}

	m.arguments = append(m.arguments, binds...)
	return nil
}

func (m *Menu[T]) buildPromptArgs() error {
	// check if prompt already set with `WithPrompt` OptFn
	hasPrompt := slices.ContainsFunc(m.arguments, func(a string) bool {
		return strings.HasPrefix(a, "--prompt")
	})
	if hasPrompt {
		return nil
	}

	prompt, err := shellwords.Parse(fmt.Sprintf("%s=%q", "--prompt", m.cfg.Prompt))
	if err != nil {
		return fmt.Errorf("parse prompt %w", err)
	}

	m.arguments = append(m.arguments, prompt...)

	return nil
}

func (m *Menu[T]) buildPreviewArgs() error {
	if m.previewCmd == "" {
		return nil
	}

	k := m.cfg.BuiltinKeymaps.Preview
	if !k.Enabled {
		return nil
	}

	k.Action = "toggle-preview"
	m.keymaps.register(k)

	args, err := m.previewArgs()
	if err != nil {
		return fmt.Errorf("parsing preview command: %w", err)
	}

	m.arguments = append(m.arguments, args...)
	return nil
}

func (m *Menu[T]) previewArgs() ([]string, error) {
	args := make([]string, 0, 3)

	if !m.withColor {
		args = append(args, "--no-color")
	}

	cmd, err := shellwords.Parse(fmt.Sprintf("%s=%q", "--preview", m.previewCmd))
	if err != nil {
		return nil, err
	}

	args = append(args, cmd...)
	args = append(args, previewWindowArg(m.cfg.Preview))

	return args, nil
}

func previewWindowArg(show bool) string {
	if show {
		return "--preview-window=~4,+{2}+4/3,<80(up)"
	}

	return "--preview-window=hidden,up"
}
