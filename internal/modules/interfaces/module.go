// Package interfaces provides a stub implementation of the /interface tree node.
// It holds a hardcoded list of known interfaces for use by other modules
// (e.g., /ip/address) that need to validate interface references.
package interfaces

import "sync"

// Module provides interface name lookup services.
// In production this would read from the /interface configuration tree;
// for now it uses a pre-populated list.
type Module struct {
	mu         sync.RWMutex
	interfaces []string
}

// New creates a new interface module with the given initial interface list.
func New(initialInterfaces []string) *Module {
	// Ensure a sensible default set of interfaces
	if len(initialInterfaces) == 0 {
		initialInterfaces = []string{"ether1", "ether2"}
	}
	// Make a copy to avoid external mutation
	list := make([]string, len(initialInterfaces))
	copy(list, initialInterfaces)
	return &Module{interfaces: list}
}

// NewDefault creates a new interface module with the standard hardcoded set.
func NewDefault() *Module {
	return New([]string{"ether1", "ether2", "ether3", "ether4", "bridge", "lo"})
}

// InterfaceExists returns true if the named interface is registered.
func (m *Module) InterfaceExists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, iface := range m.interfaces {
		if iface == name {
			return true
		}
	}
	return false
}

// ListInterfaces returns all known interface names as a copy.
func (m *Module) ListInterfaces() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]string, len(m.interfaces))
	copy(list, m.interfaces)
	return list
}
