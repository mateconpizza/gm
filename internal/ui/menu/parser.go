package menu

import (
	"fmt"
	"log/slog"
)

// handleFzfErr returns an error based on the exit code of fzf.
//
//	0      Normal exit
//	1      No match
//	2      Error
//	126    Permission denied error from become action
//	127    Invalid shell command for become action.
//	130    Interrupted with CTRL-C or ESC.
func handleFzfErr(retcode int) error {
	switch retcode {
	case 1:
		return ErrFzfNoMatching
	case 2:
		return ErrFzf
	case 126:
		return ErrFzfInvalidShellCommand
	case 127:
		return ErrFzfPermissionDenied
	case 130:
		return ErrFzfActionAborted
	}

	return nil
}

// buildPreviewOpts builds the preview options.
func buildPreviewOpts(cmd string) OptFn {
	preview := menuConfig.Keymaps.Preview
	if !preview.Enabled {
		return func(o *Options) {}
	}

	var opts []string
	if !colorEnabled {
		opts = append(opts, "--no-color")
	}
	opts = append(opts, "--preview="+cmd)
	if !menuConfig.Preview {
		opts = append(opts, "--preview-window=hidden,up")
	} else {
		opts = append(opts, "--preview-window=~4,+{2}+4/3,<80(up)")
	}

	return func(o *Options) {
		o.settings = append(o.settings, opts...)
		if !preview.Hidden && menuConfig.Preview {
			o.header = appendKeytoHeader(o.header, preview.Bind, "toggle-preview")
		}
		o.keybind = append(o.keybind, preview.Bind+":toggle-preview")
	}
}

// selectFromItems runs Fzf with the given items and returns the selected item/s.
func selectFromItems[T comparable](m *Menu[T]) ([]T, error) {
	if len(m.items) == 0 {
		return nil, ErrFzfNoItems
	}

	if m.preprocessor == nil {
		slog.Warn("preprocessor is nil, defaulting to 'defaultPreprocessor'")
		m.preprocessor = defaultPreprocessor
	}

	slog.Debug("menu args", "args", m.settings)

	// channels
	inputChan := formatItems(m.items, m.preprocessor)
	outputChan := make(chan string)
	resultChan := make(chan []T)

	go processOutput(m.items, m.preprocessor, outputChan, resultChan)

	// Build Fzf.Options
	options, err := m.runner.Parse(m.defaults, m.settings)
	if err != nil {
		return nil, fmt.Errorf("fzf: %w", err)
	}

	// Set up input and output channels
	options.Input = inputChan
	options.Output = outputChan

	// Run Fzf
	retcode, err := m.runner.Run(options)
	if retcode != 0 {
		// regardless of what kind of error, always call `callInterruptFn`
		err = handleFzfErr(retcode)
		m.callInterruptFn(err)

		return nil, err
	}

	close(outputChan)
	result := <-resultChan

	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return result, nil
}
