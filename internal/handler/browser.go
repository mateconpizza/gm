package handler

import (
	"github.com/haaag/gm/internal/browser"
	"github.com/haaag/gm/internal/browser/blink"
	"github.com/haaag/gm/internal/browser/gecko"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/sys/terminal"
)

// supportedBrowser represents a supported browser.
type supportedBrowser struct {
	key     string
	browser browser.Browser
}

// registeredBrowser the list of supported browsers.
var registeredBrowser = []supportedBrowser{
	{"f", gecko.New("Firefox", color.BrightOrange)},
	{"z", gecko.New("Zen", color.BrightBlack)},
	{"w", gecko.New("Waterfox", color.BrightBlue)},
	{"c", blink.New("Chromium", color.BrightBlue)},
	{"g", blink.New("Google Chrome", color.BrightYellow)},
	{"b", blink.New("Brave", color.BrightOrange)},
	{"v", blink.New("Vivaldi", color.BrightRed)},
	{"e", blink.New("Edge", color.BrightCyan)},
}

// getBrowser returns a browser by its short key.
//
// key: the first letter of the browser name.
//   - Firefox -> f
//   - Waterfox -> w
//   - Chromium -> c
//   - ...
func getBrowser(key string) (browser.Browser, bool) {
	for _, pair := range registeredBrowser {
		if pair.key == key {
			return pair.browser, true
		}
	}

	return nil, false
}

// selectBrowser returns the key of the browser selected by the user.
func selectBrowser(t *terminal.Term) string {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header("Supported Browsers\n").Row("\n")

	for _, c := range registeredBrowser {
		b := c.browser
		f.Mid(b.Color(b.Short()) + " " + b.Name() + "\n")
	}
	f.Row("\n").Footer("which browser do you use?")
	defer t.ClearLine(format.CountLines(f.String()))
	f.Flush()

	return t.Prompt(" ")
}
