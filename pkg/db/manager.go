package db

import (
	"log/slog"
	"sync"
)

// manager tracks and manages open SQLite connections.
var manager = NewManager()

// Manager tracks and manages open SQLite connections.
type Manager struct {
	mu    sync.RWMutex
	conns map[string]*SQLite
}

// NewManager creates a new database manager.
func NewManager() *Manager {
	return &Manager{conns: make(map[string]*SQLite)}
}

// Register adds a connection to the manager.
func (m *Manager) Register(s string, conn *SQLite) {
	slog.Debug("db manager register", "path", s)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conns[s] = conn
}

// Unregister removes a connection from the manager.
func (m *Manager) Unregister(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, s)
}

// CloseAll closes all tracked connections.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	conns := make([]*SQLite, 0, len(m.conns))
	for _, c := range m.conns {
		conns = append(conns, c)
	}
	m.mu.Unlock()

	for _, c := range conns {
		c.Close()
	}

	m.mu.Lock()
	m.conns = make(map[string]*SQLite)
	m.mu.Unlock()
}

// HasOpenConnections checks if any are open.
func (m *Manager) HasOpenConnections() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.conns) > 0
}

// Shutdown closes all open connections and logs warnings if any were left open.
func Shutdown() {
	if manager.HasOpenConnections() {
		slog.Warn("unclosed database connections detected")
		manager.CloseAll()
	}
}
