package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/term"
)

var ErrNoStateToRestore = errors.New("no term state to restore")

type (
	saveStateFunc    func() (*term.State, error)
	restoreStateFunc func(state *term.State) error

	State struct {
		current     *term.State
		saveFunc    saveStateFunc
		restoreFunc restoreStateFunc
		logger      *slog.Logger
	}
)

func NewState() *State {
	return &State{
		saveFunc: func() (*term.State, error) {
			return term.GetState(int(os.Stdin.Fd()))
		},
		restoreFunc: func(s *term.State) error {
			return term.Restore(int(os.Stdin.Fd()), s)
		},
		logger: slog.Default(),
	}
}

func (s *State) Current() *term.State { return s.current }
func (s *State) WithSaveFunc(fn saveStateFunc) *State {
	s.saveFunc = fn
	return s
}

func (s *State) WithRestoreFunc(fn restoreStateFunc) *State {
	s.restoreFunc = fn
	return s
}

func (s *State) WithLogger(l *slog.Logger) *State {
	s.logger = l
	return s
}

func (s *State) Save() error {
	l := s.log()
	l.Debug("saving terminal state")
	oldState, err := s.saveFunc()
	if err != nil {
		l.Error("failed to save terminal state", "error", err)
		return fmt.Errorf("saving state: %w", err)
	}
	s.current = oldState
	l.Debug("terminal state successfully saved")
	return nil
}

func (s *State) Restore() error {
	l := s.log()
	l.Debug("restoring terminal state")
	if s.current == nil {
		l.Warn("no terminal state to restore")
		return ErrNoStateToRestore
	}
	if err := s.restoreFunc(s.current); err != nil {
		l.Error("failed to restore terminal state", "error", err)
		return fmt.Errorf("restoring state: %w", err)
	}
	l.Debug("terminal state successfully restored")
	return nil
}

func (s *State) log() *slog.Logger {
	if s.logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.logger
}
