package menu

import fzf "github.com/junegunn/fzf/src"

// MenuRunner defines the interface for running FZF with options and parsing
// arguments.
type MenuRunner interface {
	// Run executes FZF with the given options and returns the exit code.
	Run(options *fzf.Options) (int, error)

	// Parse converts command line arguments to FZF options, optionally applying
	// defaults.
	Parse(defaults bool, args Args) (*fzf.Options, error)
}

type defaultRunner struct{}

func (d *defaultRunner) Run(options *fzf.Options) (int, error) {
	return fzf.Run(options)
}

func (d *defaultRunner) Parse(def bool, args Args) (*fzf.Options, error) {
	return fzf.ParseOptions(def, args)
}
