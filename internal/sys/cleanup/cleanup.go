// Package cleanup registers functions to run at program exit in LIFO order.
package cleanup

import (
	"log/slog"
	"sync"
)

var (
	// cleanupFuncs holds functions to be executed before program termination.
	// Functions are executed in reverse order of registration (LIFO).
	cleanupFuncs []func() error

	// cleanupMu protects concurrent access to cleanupFuncs.
	cleanupMu sync.Mutex
)

// Register registers a function to be called during program cleanup.
func Register(fn func() error) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFuncs = append(cleanupFuncs, fn)
}

// Run executes all registered cleanup functions in reverse order.
func Run() {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		if err := cleanupFuncs[i](); err != nil {
			slog.Error("cleanup", "error", err)
		}
	}
}
