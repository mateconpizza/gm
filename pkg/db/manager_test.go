package db

import (
	"sync"
	"testing"
	"time"
)

func TestManager_RegisterAndCloseAll(t *testing.T) {
	t.Parallel()

	db1 := setupTestDB(t)
	defer teardownthewall(db1.DB)

	db2 := setupTestDB(t)
	defer teardownthewall(db2.DB)

	mgr := NewManager()

	mgr.Register(db1.Name(), db1)
	mgr.Register(db2.Name(), db2)

	if !mgr.HasOpenConnections() {
		t.Fatal("expected open connections after register")
	}

	mgr.CloseAll()

	if mgr.HasOpenConnections() {
		t.Fatal("expected no open connections after CloseAll")
	}
}

func TestManager_Unregister(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	db := setupTestDB(t)
	mgr.Register(t.Name(), db)
	mgr.Unregister(t.Name())

	if mgr.HasOpenConnections() {
		t.Fatal("expected no open connections after Unregister")
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	const n = 50

	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			db := setupTestDB(t)
			mgr.Register(db.Name(), db)
			if i%5 == 0 {
				mgr.Unregister(db.Name())
			}
		}(i)
	}
	wg.Wait()

	mgr.CloseAll()
	if mgr.HasOpenConnections() {
		t.Fatal("expected all closed after concurrent CloseAll")
	}
}

func TestManager_DoubleCloseSafety(t *testing.T) {
	t.Parallel()
	mgr := NewManager()
	db := setupTestDB(t)
	mgr.Register("dup", db)

	mgr.CloseAll()
	// Calling CloseAll again must be safe
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("CloseAll should not panic on second call:", r)
		}
	}()
	mgr.CloseAll()
}

// TestCloseAll ensures all open connections close cleanly without deadlocks.
func TestManager_CloseAll(t *testing.T) {
	t.Parallel()
	mgr := NewManager()

	// Open a few test DBs and register them
	for range 3 {
		r := setupTestDB(t)
		mgr.Register(r.Cfg.Name, r)
	}

	done := make(chan struct{})

	// Run CloseAll() in a goroutine — if it hangs, test will timeout
	go func() {
		mgr.CloseAll()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("CloseAll() timed out — possible deadlock")
	}

	// Verify registry is empty
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	if len(mgr.conns) != 0 {
		t.Fatalf("expected registry to be empty, got %d", len(mgr.conns))
	}
}
