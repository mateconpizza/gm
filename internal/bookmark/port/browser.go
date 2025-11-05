package port

import (
	"github.com/mateconpizza/gm/internal/sys/browser"
	"github.com/mateconpizza/gm/internal/sys/browser/blink"
	"github.com/mateconpizza/gm/internal/sys/browser/gecko"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
)

// supportedBrowser represents a supported browser.
type supportedBrowser struct {
	key     string
	browser browser.Browser
}

// registeredBrowser the list of supported browsers.
var registeredBrowser = []supportedBrowser{
	{"f", gecko.New("Firefox", ansi.BrightYellow.With(ansi.Bold))},
	{"z", gecko.New("Zen", ansi.Red.With(ansi.Bold))},
	{"w", gecko.New("Waterfox", ansi.BrightBlue.With(ansi.Bold))},
	{"c", blink.New("Chromium", ansi.BrightBlue.With(ansi.Bold))},
	{"g", blink.New("Google Chrome", ansi.BrightYellow.With(ansi.Bold))},
	{"b", blink.New("Brave", ansi.Magenta.With(ansi.Bold))},
	{"v", blink.New("Vivaldi", ansi.BrightRed.With(ansi.Bold))},
	{"e", blink.New("Edge", ansi.BrightCyan.With(ansi.Bold))},
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
func selectBrowser(c *ui.Console) string {
	f := c.Frame()
	f.Header("Supported Browsers\n").Rowln()

	for _, browser := range registeredBrowser {
		b := browser.browser
		f.Midln(b.Color(b.Short()) + " " + b.Name())
	}

	defer c.ClearLine(txt.CountLines(f.String()) + 1)
	f.Rowln().Flush()

	return c.Prompt("which browser do you use? ")
}
