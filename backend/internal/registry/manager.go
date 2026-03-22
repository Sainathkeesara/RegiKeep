package registry

import (
	"fmt"
	"sync"
)

// Manager holds all registered adapters and routes calls to the correct one.
type Manager struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{adapters: make(map[string]Adapter)}
}

// Register adds or replaces an adapter.
func (m *Manager) Register(a Adapter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.adapters[a.ID()] = a
}

// Get returns the adapter for the given registry ID.
func (m *Manager) Get(id string) (Adapter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.adapters[id]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for registry %q", id)
	}
	return a, nil
}

// List returns a snapshot of all registered adapters.
func (m *Manager) List() []Adapter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Adapter, 0, len(m.adapters))
	for _, a := range m.adapters {
		out = append(out, a)
	}
	return out
}

// IDs returns all registered registry IDs.
func (m *Manager) IDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.adapters))
	for id := range m.adapters {
		out = append(out, id)
	}
	return out
}
