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

	// runOnce guarantees cleanup only runs once.
	runOnce sync.Once
)

// Register registers a function to be called during program cleanup.
func Register(fn func() error) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFuncs = append(cleanupFuncs, fn)
}

// Run executes all registered cleanup functions in reverse order.
func Run() {
	runOnce.Do(func() {
		cleanupMu.Lock()

		// Copy slice so we can release the lock before execution.
		funcs := slices.Clone(cleanupFuncs)

		cleanupMu.Unlock()

		for _, fn := range slices.Backward(funcs) {
			if err := fn(); err != nil {
				slog.Error("cleanup", "error", err)
			}
		}
	})
}
