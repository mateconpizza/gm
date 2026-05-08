// Package cleanup registers functions to run at program exit in LIFO order.
package cleanup

import (
	"log/slog"
	"slices"
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
	for _, v := range slices.Backward(cleanupFuncs) {
		if err := v(); err != nil {
			slog.Error("cleanup", "error", err)
		}
	}
}
