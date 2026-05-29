// Package bridge_port implements the /interface bridge port settings directory for RouterOS v7.
// It provides adding and removing physical interfaces to/from a bridge.
package bridge_port

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Devaste/MikroLab/internal/core"
)

// ---------------------------------------------------------------------------
// bridgePortEntry — internal representation
// ---------------------------------------------------------------------------

// bridgePortEntry stores all properties for a single bridge port entry.
type bridgePortEntry struct {
	id           string
	index        int
	ifaceName    string
	bridgeName   string
	pathCost     int
	priority     int
	edge         string // "auto", "yes", "no"
	pointToPoint string // "auto", "yes", "no"
	disabled     bool
	status       string // "in-bridge" or "inactive"
}

// ---------------------------------------------------------------------------
// entryAdapter — wraps bridgePortEntry to satisfy core.Entry
// ---------------------------------------------------------------------------

type entryAdapter struct {
	entry *bridgePortEntry
}

func (a *entryAdapter) ID() string     { return a.entry.id }
func (a *entryAdapter) Index() int     { return a.entry.index }
func (a *entryAdapter) Disabled() bool { return a.entry.disabled }
func (a *entryAdapter) Dynamic() bool  { return false }
func (a *entryAdapter) Invalid() bool  { return false }
func (a *entryAdapter) Slave() bool    { return false }

func (a *entryAdapter) Property(name string) (interface{}, bool) {
	switch name {
	case "interface":
		return a.entry.ifaceName, true
	case "bridge":
		return a.entry.bridgeName, true
	case "path-cost":
		return a.entry.pathCost, true
	case "priority":
		return a.entry.priority, true
	case "edge":
		return a.entry.edge, true
	case "point-to-point":
		return a.entry.pointToPoint, true
	case "disabled":
		return a.entry.disabled, true
	case "status":
		return a.entry.status, true
	default:
		return nil, false
	}
}

func (a *entryAdapter) SetProperty(name string, value interface{}) error {
	switch name {
	case "path-cost":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("path-cost: %w", err)
		}
		if i < 0 {
			return fmt.Errorf("path-cost must be non-negative")
		}
		a.entry.pathCost = i
	case "priority":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("priority: %w", err)
		}
		if i < 0 || i > 255 {
			return fmt.Errorf("priority must be between 0 and 255")
		}
		a.entry.priority = i
	case "edge":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("edge must be a string")
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "auto", "yes", "no":
			a.entry.edge = s
		default:
			return fmt.Errorf("edge must be one of: auto, yes, no")
		}
	case "point-to-point":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("point-to-point must be a string")
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "auto", "yes", "no":
			a.entry.pointToPoint = s
		default:
			return fmt.Errorf("point-to-point must be one of: auto, yes, no")
		}
	case "disabled":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("disabled: %w", err)
		}
		a.entry.disabled = b
	case "interface":
		return fmt.Errorf("interface is read-only after creation")
	case "bridge":
		return fmt.Errorf("bridge is read-only after creation")
	default:
		return fmt.Errorf("property %q is read-only or does not exist", name)
	}
	return nil
}

func (a *entryAdapter) Properties() map[string]interface{} {
	return map[string]interface{}{
		"interface":      a.entry.ifaceName,
		"bridge":         a.entry.bridgeName,
		"path-cost":      a.entry.pathCost,
		"priority":       a.entry.priority,
		"edge":           a.entry.edge,
		"point-to-point": a.entry.pointToPoint,
		"disabled":       a.entry.disabled,
		"status":         a.entry.status,
	}
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"disabled": a.entry.disabled,
		"running":  a.entry.status == "in-bridge",
		"dynamic":  false,
	}
}

// Compile-time check
var _ core.Entry = (*entryAdapter)(nil)

// ---------------------------------------------------------------------------
// Dependencies
// ---------------------------------------------------------------------------

// InterfaceChecker provides interface name validation and listing.
type InterfaceChecker interface {
	InterfaceExists(name string) bool
	ListInterfaces() []string
}

// BridgeManager provides bridge management operations.
type BridgeManager interface {
	// BridgeExists checks if a bridge with the given name exists.
	BridgeExists(name string) bool
	AddPort(bridgeName, portName string) error
	RemovePort(bridgeName, portName string) error
	HasPort(portName string) (bridgeName string, found bool)
}

// ---------------------------------------------------------------------------
// BridgePortModule — implements core.SettingsDirectory
// ---------------------------------------------------------------------------

// BridgePortModule provides full CRUD for /interface bridge port entries.
type BridgePortModule struct {
	mu    sync.RWMutex
	path  string
	title string

	entries      map[string]*bridgePortEntry // id -> entry
	byIface      map[string]string           // interface name -> entry id
	index        int
	ifaceChecker InterfaceChecker
	bridgeMgr    BridgeManager
}

// New creates a new BridgePortModule.
func New(path string, title string, ifaceChecker InterfaceChecker, bridgeMgr BridgeManager) (*BridgePortModule, error) {
	if path == "" {
		return nil, fmt.Errorf("bridge_port: path is required")
	}
	if ifaceChecker == nil {
		return nil, fmt.Errorf("bridge_port: interface checker is required")
	}
	if bridgeMgr == nil {
		return nil, fmt.Errorf("bridge_port: bridge manager is required")
	}
	return &BridgePortModule{
		path:         path,
		title:        title,
		entries:      make(map[string]*bridgePortEntry),
		byIface:      make(map[string]string),
		ifaceChecker: ifaceChecker,
		bridgeMgr:    bridgeMgr,
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

func (m *BridgePortModule) Path() string        { return m.path }
func (m *BridgePortModule) Type() core.NodeType { return core.NodeTypeList }
func (m *BridgePortModule) Title() string       { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

func (m *BridgePortModule) Children() map[string]core.Node { return nil }
func (m *BridgePortModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("bridge_port: list node %q cannot accept child %q", m.path, name)
}
func (m *BridgePortModule) RemoveChild(name string) error {
	return fmt.Errorf("bridge_port: list node %q has no child %q", m.path, name)
}
func (m *BridgePortModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new bridge port entry.
func (m *BridgePortModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Extract and validate interface name
	ifaceRaw, hasIface := props["interface"]
	if !hasIface {
		return nil, fmt.Errorf("bridge_port: required property %q is missing", "interface")
	}
	ifaceName, ok := ifaceRaw.(string)
	if !ok || strings.TrimSpace(ifaceName) == "" {
		return nil, fmt.Errorf("bridge_port: property %q must be a non-empty string", "interface")
	}
	ifaceName = strings.TrimSpace(ifaceName)

	// 2. Validate interface exists
	if !m.ifaceChecker.InterfaceExists(ifaceName) {
		return nil, fmt.Errorf("bridge_port: interface %q does not exist", ifaceName)
	}

	// 3. Check interface not already in a bridge
	if existingID, found := m.byIface[ifaceName]; found {
		if existing, exists := m.entries[existingID]; exists {
			return nil, fmt.Errorf("bridge_port: interface %q is already a port of bridge %q",
				ifaceName, existing.bridgeName)
		}
	}

	// Also check via bridge manager if already registered elsewhere
	if bridgeName, found := m.bridgeMgr.HasPort(ifaceName); found {
		return nil, fmt.Errorf("bridge_port: interface %q is already a port of bridge %q",
			ifaceName, bridgeName)
	}

	// 4. Extract and validate bridge name
	bridgeRaw, hasBridge := props["bridge"]
	if !hasBridge {
		return nil, fmt.Errorf("bridge_port: required property %q is missing", "bridge")
	}
	bridgeName, ok := bridgeRaw.(string)
	if !ok || strings.TrimSpace(bridgeName) == "" {
		return nil, fmt.Errorf("bridge_port: property %q must be a non-empty string", "bridge")
	}
	bridgeName = strings.TrimSpace(bridgeName)

	// 5. Validate bridge exists
	if !m.bridgeMgr.BridgeExists(bridgeName) {
		return nil, fmt.Errorf("bridge_port: bridge %q does not exist", bridgeName)
	}

	// 6. Build entry with defaults
	entry := &bridgePortEntry{
		id:           strconv.Itoa(m.index),
		index:        m.index,
		ifaceName:    ifaceName,
		bridgeName:   bridgeName,
		pathCost:     10,
		priority:     128,
		edge:         "auto",
		pointToPoint: "auto",
		disabled:     false,
		status:       "in-bridge",
	}
	m.index++

	// 7. Apply overrides from props
	if err := applyPortProps(entry, props); err != nil {
		return nil, err
	}

	// 8. Register port with bridge
	if err := m.bridgeMgr.AddPort(bridgeName, ifaceName); err != nil {
		return nil, fmt.Errorf("bridge_port: failed to register port with bridge: %w", err)
	}

	// 9. Store
	m.entries[entry.id] = entry
	m.byIface[ifaceName] = entry.id

	return &entryAdapter{entry: entry}, nil
}

// Set updates properties of an existing bridge port.
func (m *BridgePortModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("bridge_port: entry %q not found", id)
	}

	// If interface is being changed, validate uniqueness
	if ifaceRaw, hasIface := props["interface"]; hasIface {
		newIface, ok := ifaceRaw.(string)
		if !ok || strings.TrimSpace(newIface) == "" {
			return fmt.Errorf("bridge_port: property %q must be a non-empty string", "interface")
		}
		newIface = strings.TrimSpace(newIface)
		if newIface != entry.ifaceName {
			// Validate interface exists
			if !m.ifaceChecker.InterfaceExists(newIface) {
				return fmt.Errorf("bridge_port: interface %q does not exist", newIface)
			}
			// Check not already in a bridge
			if existingID, found := m.byIface[newIface]; found && existingID != id {
				if existing, exists := m.entries[existingID]; exists {
					return fmt.Errorf("bridge_port: interface %q is already a port of bridge %q",
						newIface, existing.bridgeName)
				}
			}
			if bridgeName, found := m.bridgeMgr.HasPort(newIface); found {
				if existing, ok := m.byIface[newIface]; !ok || existing != id {
					return fmt.Errorf("bridge_port: interface %q is already a port of bridge %q",
						newIface, bridgeName)
				}
			}
			// Remove old interface from bridge
			if err := m.bridgeMgr.RemovePort(entry.bridgeName, entry.ifaceName); err != nil {
				return err
			}
			// Update maps
			delete(m.byIface, entry.ifaceName)
			entry.ifaceName = newIface
			m.byIface[newIface] = id
			// Add new interface to bridge
			if err := m.bridgeMgr.AddPort(entry.bridgeName, newIface); err != nil {
				return err
			}
		}
		delete(props, "interface")
	}

	// If bridge is being changed, update registration
	if bridgeRaw, hasBridge := props["bridge"]; hasBridge {
		newBridge, ok := bridgeRaw.(string)
		if !ok || strings.TrimSpace(newBridge) == "" {
			return fmt.Errorf("bridge_port: property %q must be a non-empty string", "bridge")
		}
		newBridge = strings.TrimSpace(newBridge)
		if newBridge != entry.bridgeName {
			if !m.bridgeMgr.BridgeExists(newBridge) {
				return fmt.Errorf("bridge_port: bridge %q does not exist", newBridge)
			}
			// Remove from old bridge, add to new
			if err := m.bridgeMgr.RemovePort(entry.bridgeName, entry.ifaceName); err != nil {
				return err
			}
			entry.bridgeName = newBridge
			if err := m.bridgeMgr.AddPort(newBridge, entry.ifaceName); err != nil {
				return err
			}
		}
		delete(props, "bridge")
	}

	if err := applyPortProps(entry, props); err != nil {
		return err
	}

	return nil
}

// Remove deletes a bridge port entry.
func (m *BridgePortModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("bridge_port: entry %q not found", id)
	}

	// Unregister from bridge
	if err := m.bridgeMgr.RemovePort(entry.bridgeName, entry.ifaceName); err != nil {
		return err
	}

	delete(m.entries, id)
	delete(m.byIface, entry.ifaceName)
	return nil
}

// List returns all entries sorted by index.
func (m *BridgePortModule) List() []core.Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]core.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, &entryAdapter{entry: e})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Index() < result[j].Index()
	})
	return result
}

// Get returns the entry with the given ID, or false if not found.
func (m *BridgePortModule) Get(id string) (core.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[id]
	if !exists {
		return nil, false
	}
	return &entryAdapter{entry: entry}, true
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func applyPortProps(entry *bridgePortEntry, props map[string]interface{}) error {
	for name, rawVal := range props {
		switch name {
		case "path-cost":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("bridge_port: path-cost: %w", err)
			}
			if i < 0 {
				return fmt.Errorf("bridge_port: path-cost must be non-negative")
			}
			entry.pathCost = i
		case "priority":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("bridge_port: priority: %w", err)
			}
			if i < 0 || i > 255 {
				return fmt.Errorf("bridge_port: priority must be between 0 and 255")
			}
			entry.priority = i
		case "edge":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("bridge_port: edge must be a string")
			}
			s = strings.ToLower(strings.TrimSpace(s))
			switch s {
			case "auto", "yes", "no":
				entry.edge = s
			default:
				return fmt.Errorf("bridge_port: edge must be one of: auto, yes, no")
			}
		case "point-to-point":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("bridge_port: point-to-point must be a string")
			}
			s = strings.ToLower(strings.TrimSpace(s))
			switch s {
			case "auto", "yes", "no":
				entry.pointToPoint = s
			default:
				return fmt.Errorf("bridge_port: point-to-point must be one of: auto, yes, no")
			}
		case "disabled":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("bridge_port: disabled: %w", err)
			}
			entry.disabled = b
		case "interface", "bridge":
			// handled by caller
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Type conversion helpers
// ---------------------------------------------------------------------------

func toInt(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(val))
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}

func toBool(v interface{}) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true", "yes", "1":
			return true, nil
		case "false", "no", "0":
			return false, nil
		default:
			return false, fmt.Errorf("invalid boolean %q", val)
		}
	case float64:
		return val != 0, nil
	default:
		return false, fmt.Errorf("expected boolean, got %T", v)
	}
}
