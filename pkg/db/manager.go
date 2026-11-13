package db

import (
	"log/slog"
	"sync"
)

// manager tracks and manages open SQLite connections.
var manager = NewManager()

// Manager tracks and manages open SQLite connections.
type Manager struct {
	mu     sync.RWMutex
	conns  map[string]*SQLite
	logger *slog.Logger
}

// NewManager creates a new database manager.
func NewManager() *Manager {
	return &Manager{conns: make(map[string]*SQLite)}
}

// WithLogger sets the logger, tagging it with the component name.
func (m *Manager) WithLogger(l *slog.Logger) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = l.With("component", "db-manager")
	return m
}

// Register adds a connection to the manager.
func (m *Manager) Register(s string, conn *SQLite) {
	m.mu.Lock()
	m.conns[s] = conn
	n := len(m.conns)
	l := m.log()
	m.mu.Unlock()

	l.Debug("registered connection", "name", s, "open", n)
}

// Unregister removes a connection from the manager.
func (m *Manager) Unregister(s string) {
	m.mu.Lock()
	_, ok := m.conns[s]
	delete(m.conns, s)
	n := len(m.conns)
	l := m.log()
	m.mu.Unlock()

	if !ok {
		l.Debug("unregister: connection not tracked", "name", s)
		return
	}
	l.Debug("unregistered connection", "name", s, "open", n)
}

// CloseAll closes all tracked connections.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	conns := make([]*SQLite, 0, len(m.conns))
	for _, c := range m.conns {
		conns = append(conns, c)
	}
	l := m.log()
	m.mu.Unlock()

	l.Debug("closing all connections", "count", len(conns))

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

// log returns the configured logger, or the current default if none was set.
// Callers must hold m.mu (read or write).
func (m *Manager) log() *slog.Logger {
	if m.logger != nil {
		return m.logger
	}
	return slog.Default().With("component", "db-manager")
}

// Shutdown closes all open connections and logs warnings if any were left open.
func Shutdown() {
	manager.mu.RLock()
	open := len(manager.conns)
	l := manager.log()
	manager.mu.RUnlock()

	if open > 0 {
		l.Warn("unclosed database connections detected", "count", open)
		manager.CloseAll()
	}
}
