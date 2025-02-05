// Spinner displays a spinning cursor animation while waiting for a signal on a
// channel.
package spinner

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/haaag/gm/internal/format/color"
)

// OptFn is an option function for the spinner.
type OptFn func(*Options)

// Options represents the options for the spinner.
type Options struct {
	mu      *sync.RWMutex
	done    chan bool
	color   color.ColorFn
	Mesg    string
	unicode []string
	started bool
}

// Spinner represents a CLI spinner animation.
type Spinner struct {
	Options
}

// Start starts the spinning animation in a goroutine.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Print("spinner started")
	s.started = true

	go func() {
		for i := 0; ; i++ {
			select {
			case <-s.done:
				// Clear the spinner and message
				fmt.Printf("\r%s\r", strings.Repeat(" ", len(s.Mesg)+4))
				return
			default:
				// Print the spinner animation
				fmt.Printf("\r%s %s", s.color(s.unicode[i%len(s.unicode)]).String(), s.Mesg)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner animation.
func (s *Spinner) Stop() {
	if !s.started {
		log.Print("spinner not started")
		return
	}
	s.done <- true
	time.Sleep(100 * time.Millisecond)
	log.Print("spinner stopped")
}

// defaultOpts returns the default spinner options.
func defaultOpts() Options {
	return Options{
		Mesg:    "Loading...",
		unicode: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:    make(chan bool),
		mu:      &sync.RWMutex{},
		started: false,
		color:   color.BrightWhite,
	}
}

// WithUnicode returns an option function that sets the spinner unicode
// animation.
func WithUnicode(unicode []string) OptFn {
	return func(o *Options) {
		o.unicode = unicode
	}
}

// WithUnicode returns an option function that sets the spinner unicode
// animation with blocks.
func WithUnicodeBlock() OptFn {
	return func(o *Options) {
		o.unicode = []string{"░", "▒", "▒", "░", "▓"}
	}
}

// WithUnicodeDots returns an option function that sets the spinner unicode
// animation with dots.
func WithUnicodeDots() OptFn {
	return func(o *Options) {
		o.unicode = []string{
			"  . . . .",
			".   . . .",
			". .   . .",
			". . .   .",
			". . . .  ",
			". . . . .",
		}
	}
}

// WithUnicodeBar returns an option function that sets the spinner unicode
// animation with bars.
func WithUnicodeBar() OptFn {
	return func(o *Options) {
		o.unicode = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	}
}

// WithColor returns an option function.
func WithColor(c color.ColorFn) OptFn {
	return func(o *Options) {
		o.color = c
	}
}

// WithMesg returns an option function that sets the spinner message.
func WithMesg(s string) OptFn {
	return func(o *Options) {
		o.Mesg = s
	}
}

// New returns a new spinner.
func New(opt ...OptFn) *Spinner {
	o := defaultOpts()
	for _, fn := range opt {
		fn(&o)
	}

	return &Spinner{
		Options: o,
	}
}
