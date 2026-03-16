package menu

import "strings"

// Args holds the FZF arguments.
type Args []string

// ArgsBuilder constructs command-line arguments for FZF.
type ArgsBuilder struct {
	list          Args
	ansi          string // Enable processing of ANSI color codes
	bind          string // Comma-separated list of custom key/event bindings
	border        string // Border around the window
	borderLabel   string // Label to print on the horizontal border line
	color         string // Color configuration
	footer        string // The given string will be printed as the sticky footer
	header        string // The given string will be printed as the sticky header
	height        string // Set the height of the menu
	highlightLine string // Highlight the whole current line (bold)
	info          string // Determines the display style of the finder info.
	layout        string // Choose the layout (default: default)
	multi         string // Enable multi-select with tab/shift-tab
	noColor       string // Disable output color
	noScrollbar   string // Remove scrollbar
	pointer       string // Pointer to the current line
	preview       string // Execute the given command for the current line
	previewWindow string // Determines the layout of the preview window.
	prompt        string // Input prompt
	read0         string // Read input delimited by ASCII NUL characters instead of newline characters
	sync          string // Synchronous search for multi-staged filtering
	tac           string // Reverse the order of the input
	cycle         string // Enable cyclic scroll
}

func (a *ArgsBuilder) add(s ...string) *ArgsBuilder {
	a.list = append(a.list, s...)
	return a
}

func (a *ArgsBuilder) build() Args                       { return a.list }
func (a *ArgsBuilder) withAnsi() *ArgsBuilder            { return a.add(a.ansi) }
func (a *ArgsBuilder) withHeight(s string) *ArgsBuilder  { return a.add(a.height + "=" + s) }
func (a *ArgsBuilder) withInfo(s string) *ArgsBuilder    { return a.add(a.info + "=" + s) }
func (a *ArgsBuilder) withLayout(s string) *ArgsBuilder  { return a.add(a.layout + "=" + s) }
func (a *ArgsBuilder) withNoColor() *ArgsBuilder         { return a.add(a.noColor) }
func (a *ArgsBuilder) withNoScrollbar() *ArgsBuilder     { return a.add(a.noScrollbar) }
func (a *ArgsBuilder) withPointer(s string) *ArgsBuilder { return a.add(a.pointer + "=" + s) }
func (a *ArgsBuilder) withPreview(s string) *ArgsBuilder { return a.add(a.preview + "=" + s) }
func (a *ArgsBuilder) withPrompt(s string) *ArgsBuilder  { return a.add(a.prompt + "=" + s) }
func (a *ArgsBuilder) withSync() *ArgsBuilder            { return a.add(a.sync) }
func (a *ArgsBuilder) withTac() *ArgsBuilder             { return a.add(a.tac) }
func (a *ArgsBuilder) withCycle() *ArgsBuilder           { return a.add(a.cycle) }

func (a *ArgsBuilder) withBorderLabel(s string) *ArgsBuilder {
	return a.add(a.border, a.borderLabel+"="+s)
}

func (a *ArgsBuilder) withColor(target string, styles ...string) *ArgsBuilder {
	// TODO: expose this as `Menu Option`
	color := a.color + "=" + target
	if len(styles) > 0 {
		color += ":" + strings.Join(styles, ":")
	}

	return a.add(color)
}

func newArgsBuilder() *ArgsBuilder {
	return &ArgsBuilder{
		ansi:          "--ansi",
		bind:          "--bind",
		border:        "--border",
		borderLabel:   "--border-label",
		color:         "--color",
		footer:        "--footer",
		header:        "--header",
		height:        "--height",
		highlightLine: "--highlight-line",
		info:          "--info",
		layout:        "--layout",
		multi:         "--multi",
		noColor:       "--no-color",
		noScrollbar:   "--no-scrollbar",
		pointer:       "--pointer",
		preview:       "--preview",
		previewWindow: "--preview-window",
		prompt:        "--prompt",
		read0:         "--read0",
		sync:          "--sync",
		tac:           "--tac",
		cycle:         "--cycle",
	}
}
