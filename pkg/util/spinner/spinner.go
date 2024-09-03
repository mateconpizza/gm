// Spinner displays a spinning cursor animation while waiting for a signal on a
// channel.
package spinner

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// OptFn is an option function for the spinner.
type OptFn func(*Options)

// Options represents the options for the spinner.
type Options struct {
	mu      *sync.RWMutex
	done    chan bool
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
				fmt.Printf("\r%s\r", strings.Repeat(" ", len(s.Mesg)+4))
				return
			default:
				fmt.Printf("\r%s %s", s.unicode[i%len(s.unicode)], s.Mesg)
				time.Sleep(110 * time.Millisecond)
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
	}
}

// WithUnicode returns an option function that sets the spinner unicode
// animation.
func WithUnicode(unicode []string) OptFn {
	return func(o *Options) {
		o.unicode = unicode
	}
}

// WithMesg returns an option function that sets the spinner message.
func WithMesg(mesg string) OptFn {
	return func(o *Options) {
		o.Mesg = mesg
	}
}

// New returns a new spinner.
func New(opts ...OptFn) *Spinner {
	o := defaultOpts()
	for _, fn := range opts {
		fn(&o)
	}

	return &Spinner{
		Options: o,
	}
}
