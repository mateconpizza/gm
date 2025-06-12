package port

import (
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/browser/blink"
	"github.com/mateconpizza/gm/internal/sys/browser/gecko"
	"github.com/mateconpizza/gm/internal/format"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
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
	if key == "" {
		return nil, false
	}
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
	f.Header("Supported Browsers\n").Rowln()

	for _, c := range registeredBrowser {
		b := c.browser
		f.Midln(b.Color(b.Short()) + " " + b.Name())
	}
	f.Rowln().Footer("which browser do you use?")
	defer t.ClearLine(format.CountLines(f.String()))
	f.Flush()

	return t.Prompt(" ")
}
