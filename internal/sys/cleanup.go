package sys

import "sync"

var (
	// cleanupFuncs holds functions to be executed before program termination.
	// Functions are executed in reverse order of registration (LIFO).
	cleanupFuncs []func()

	// cleanupMu protects concurrent access to cleanupFuncs.
	cleanupMu sync.Mutex
)

// RegisterCleanup registers a function to be called during program cleanup.
func RegisterCleanup(fn func()) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFuncs = append(cleanupFuncs, fn)
}

// runCleanup executes all registered cleanup functions in reverse order.
func runCleanup() {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		cleanupFuncs[i]()
	}
}
