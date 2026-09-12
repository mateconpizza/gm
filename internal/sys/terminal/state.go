package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/term"
)

var ErrNoStateToRestore = errors.New("no term state to restore")

type saveStateFunc func() (*term.State, error)

type restoreStateFunc func(state *term.State) error

type State struct {
	current     *term.State
	saveFunc    saveStateFunc
	restoreFunc restoreStateFunc
}

func NewState() *State {
	return &State{
		saveFunc: func() (*term.State, error) {
			return term.GetState(int(os.Stdin.Fd()))
		},
		restoreFunc: func(s *term.State) error {
			return term.Restore(int(os.Stdin.Fd()), s)
		},
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

func (s *State) Save() error {
	slog.Debug("saving terminal state")
	oldState, err := s.saveFunc()
	if err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	s.current = oldState
	return nil
}

func (s *State) Restore() error {
	slog.Debug("restoring terminal state")
	if s.current == nil {
		return ErrNoStateToRestore
	}
	if err := s.restoreFunc(s.current); err != nil {
		return fmt.Errorf("restoring state: %w", err)
	}
	return nil
}
