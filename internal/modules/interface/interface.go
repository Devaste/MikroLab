// Package interfaces implements the /interface settings directory.
// It provides full CRUD operations on network interfaces and satisfies
// both core.SettingsDirectory and core.InterfaceChecker so that other
// modules (e.g., /ip/address) can validate interface references.
package interfaces

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
)

// ---------------------------------------------------------------------------
// interfaceEntry — internal representation of a single interface
// ---------------------------------------------------------------------------

// interfaceEntry stores all properties for a single network interface.
type interfaceEntry struct {
	id          string
	index       int
	name        string
	defaultName string
	ifaceType   string
	mtu         int
	actualMTU   int
	l2mtu       int
	macAddress  string
	running     bool
	comment     string
	disabled    bool
	dynamic     bool
}

// ---------------------------------------------------------------------------
// entryAdapter — wraps interfaceEntry to satisfy core.Entry
// ---------------------------------------------------------------------------

type entryAdapter struct {
	entry *interfaceEntry
}

func (a *entryAdapter) ID() string     { return a.entry.id }
func (a *entryAdapter) Index() int     { return a.entry.index }
func (a *entryAdapter) Disabled() bool { return a.entry.disabled }
func (a *entryAdapter) Dynamic() bool  { return a.entry.dynamic }
func (a *entryAdapter) Invalid() bool  { return false }
func (a *entryAdapter) Slave() bool    { return false }

func (a *entryAdapter) Property(name string) (interface{}, bool) {
	switch name {
	case "name":
		return a.entry.name, true
	case "default-name":
		return a.entry.defaultName, true
	case "type":
		return a.entry.ifaceType, true
	case "mtu":
		return a.entry.mtu, true
	case "actual-mtu":
		return a.entry.actualMTU, true
	case "l2mtu":
		return a.entry.l2mtu, true
	case "mac-address":
		return a.entry.macAddress, true
	case "running":
		return a.entry.running, true
	case "comment":
		return a.entry.comment, true
	case "disabled":
		return a.entry.disabled, true
	default:
		return nil, false
	}
}

func (a *entryAdapter) SetProperty(name string, value interface{}) error {
	switch name {
	case "name":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("name must be a string")
		}
		a.entry.name = s
	case "mtu":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("mtu: %w", err)
		}
		a.entry.mtu = i
		a.entry.actualMTU = i // actual-mtu follows mtu
	case "l2mtu":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("l2mtu: %w", err)
		}
		a.entry.l2mtu = i
	case "comment":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("comment must be a string")
		}
		a.entry.comment = s
	case "disabled":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("disabled: %w", err)
		}
		a.entry.disabled = b
		a.entry.running = !b // running is computed from disabled
	default:
		return fmt.Errorf("property %q is read-only or does not exist", name)
	}
	return nil
}

func (a *entryAdapter) Properties() map[string]interface{} {
	return map[string]interface{}{
		"name":         a.entry.name,
		"default-name": a.entry.defaultName,
		"type":         a.entry.ifaceType,
		"mtu":          a.entry.mtu,
		"actual-mtu":   a.entry.actualMTU,
		"l2mtu":        a.entry.l2mtu,
		"mac-address":  a.entry.macAddress,
		"running":      a.entry.running,
		"comment":      a.entry.comment,
		"disabled":     a.entry.disabled,
	}
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"disabled": a.entry.disabled,
		"running":  a.entry.running,
		"dynamic":  a.entry.dynamic,
		"inactive": !a.entry.running,
	}
}

// Compile-time check
var _ core.Entry = (*entryAdapter)(nil)

// ---------------------------------------------------------------------------
// InterfaceModule — implements core.SettingsDirectory + core.InterfaceChecker
// ---------------------------------------------------------------------------

// InterfaceModule provides full CRUD for /interface entries.
// It also acts as an InterfaceChecker so other modules (e.g., /ip/address)
// can validate that an interface name exists.
type InterfaceModule struct {
	mu     sync.RWMutex
	path   string
	title  string
	schema *config.ModuleSchema

	entries map[string]*interfaceEntry // id -> entry
	byName  map[string]string          // name -> id
	index   int
}

// New creates a new InterfaceModule with the given schema.
func New(schema *config.ModuleSchema) (*InterfaceModule, error) {
	if schema == nil {
		return nil, fmt.Errorf("interface: schema is required")
	}
	return &InterfaceModule{
		path:    schema.Path,
		title:   schema.Title,
		schema:  schema,
		entries: make(map[string]*interfaceEntry),
		byName:  make(map[string]string),
	}, nil
}

// ---------------------------------------------------------------------------
// Exists — exported helper to check if an interface name exists
// ---------------------------------------------------------------------------

// Exists returns true if an interface with the given name is registered.
func (m *InterfaceModule) Exists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.byName[name]
	return ok
}

// ---------------------------------------------------------------------------
// core.InterfaceChecker
// ---------------------------------------------------------------------------

// InterfaceExists returns true if the named interface is registered.
func (m *InterfaceModule) InterfaceExists(name string) bool {
	return m.Exists(name)
}

// ListInterfaces returns all known interface names (as a copy).
func (m *InterfaceModule) ListInterfaces() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.byName))
	for n := range m.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

func (m *InterfaceModule) Path() string        { return m.path }
func (m *InterfaceModule) Type() core.NodeType { return core.NodeTypeList }
func (m *InterfaceModule) Title() string       { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

func (m *InterfaceModule) Children() map[string]core.Node { return nil }
func (m *InterfaceModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("interface: list node %q cannot accept child %q", m.path, name)
}
func (m *InterfaceModule) RemoveChild(name string) error {
	return fmt.Errorf("interface: list node %q has no child %q", m.path, name)
}
func (m *InterfaceModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new interface entry.
func (m *InterfaceModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Extract and coerce properties
	nameRaw, hasName := props["name"]
	if !hasName {
		return nil, fmt.Errorf("interface: required property %q is missing", "name")
	}
	name, ok := nameRaw.(string)
	if !ok || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("interface: property %q must be a non-empty string", "name")
	}
	name = strings.TrimSpace(name)

	// 2. Validate unique name
	if _, exists := m.byName[name]; exists {
		return nil, fmt.Errorf("interface: interface %q already exists", name)
	}

	// 3. Build entry with defaults
	entry := &interfaceEntry{
		id:          strconv.Itoa(m.index),
		index:       m.index,
		name:        name,
		defaultName: generateDefaultName(m.index),
		ifaceType:   "ether",
		mtu:         1500,
		actualMTU:   1500,
		l2mtu:       1500,
		macAddress:  generateMAC(m.index),
		running:     true,
	}
	m.index++

	// 4. Apply overrides from props
	if err := applyProps(entry, props); err != nil {
		return nil, err
	}

	// 5. Store
	m.entries[entry.id] = entry
	m.byName[entry.name] = entry.id

	return &entryAdapter{entry: entry}, nil
}

// Set updates properties of an existing interface.
func (m *InterfaceModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("interface: entry %q not found", id)
	}

	// If name is being changed, validate uniqueness
	if nameRaw, hasName := props["name"]; hasName {
		newName, ok := nameRaw.(string)
		if !ok || strings.TrimSpace(newName) == "" {
			return fmt.Errorf("interface: property %q must be a non-empty string", "name")
		}
		newName = strings.TrimSpace(newName)
		if newName != entry.name {
			if existingID, taken := m.byName[newName]; taken && existingID != id {
				return fmt.Errorf("interface: interface %q already exists", newName)
			}
			// Update name map
			delete(m.byName, entry.name)
			entry.name = newName
			m.byName[newName] = id
		}
		// Remove from props since we already handled it
		delete(props, "name")
	}

	// Apply remaining props
	if err := applyProps(entry, props); err != nil {
		return err
	}

	return nil
}

// Remove deletes an interface entry.
func (m *InterfaceModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("interface: entry %q not found", id)
	}
	if entry.dynamic {
		return fmt.Errorf("interface: cannot remove dynamic entry %q", id)
	}

	delete(m.entries, id)
	delete(m.byName, entry.name)
	return nil
}

// List returns all entries sorted by index.
func (m *InterfaceModule) List() []core.Entry {
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
func (m *InterfaceModule) Get(id string) (core.Entry, bool) {
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

// applyProps applies writable properties to an interfaceEntry.
// Only known writable properties are applied; unknown/read-only props are
// silently ignored (caller must validate upstream if needed).
func applyProps(entry *interfaceEntry, props map[string]interface{}) error {
	for name, rawVal := range props {
		switch name {
		case "mtu":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("interface: mtu: %w", err)
			}
			entry.mtu = i
			entry.actualMTU = i
		case "l2mtu":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("interface: l2mtu: %w", err)
			}
			entry.l2mtu = i
		case "comment":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("interface: comment must be a string")
			}
			entry.comment = s
		case "disabled":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("interface: disabled: %w", err)
			}
			entry.disabled = b
			entry.running = !b
		case "name":
			// handled by caller (Set) or already set (Add)
		default:
			// ignore read-only or unknown properties
		}
	}
	return nil
}

// generateDefaultName produces "etherX" where X is the given index + 1.
func generateDefaultName(index int) string {
	return fmt.Sprintf("ether%d", index+1)
}

// generateMAC produces a deterministic MAC address for a given index.
func generateMAC(index int) string {
	// Base prefix 00:11:22:33:44, last octet varies
	octet := (index % 256)
	return fmt.Sprintf("00:11:22:33:44:%02X", octet)
}

// toInt converts an interface{} to int.
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

// toBool converts an interface{} to bool.
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
