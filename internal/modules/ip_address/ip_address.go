// Package ip_address implements the /ip/address settings directory.
package ip_address

import (
	"fmt"
	"sort"
	"sync"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	"github.com/google/uuid"
)

// entryAdapter wraps a *config.Entry to satisfy the core.Entry interface.
type entryAdapter struct {
	entry *config.Entry
}

func (a *entryAdapter) ID() string                               { return a.entry.ID }
func (a *entryAdapter) Index() int                               { return a.entry.Index }
func (a *entryAdapter) Disabled() bool                           { return a.entry.Disabled }
func (a *entryAdapter) Dynamic() bool                            { return a.entry.Dynamic }
func (a *entryAdapter) Invalid() bool                            { return a.entry.Invalid }
func (a *entryAdapter) Slave() bool                              { return a.entry.Slave }
func (a *entryAdapter) Property(name string) (interface{}, bool) { return a.entry.GetProperty(name) }
func (a *entryAdapter) SetProperty(name string, value interface{}) error {
	return a.entry.SetProperty(name, value)
}

func (a *entryAdapter) Properties() map[string]interface{} {
	props := make(map[string]interface{}, len(a.entry.Properties))
	for k, pv := range a.entry.Properties {
		props[k] = pv.Value
	}
	return props
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"disabled": a.entry.Disabled,
		"dynamic":  a.entry.Dynamic,
		"invalid":  a.entry.Invalid,
		"slave":    a.entry.Slave,
	}
}

// Compile-time check that config.Entry satisfies core.Entry through the adapter.
var _ core.Entry = (*entryAdapter)(nil)

// IPAddressModule implements the /ip/address settings directory.
// It maintains an in-memory map of entries and delegates schema validation
// and sanitization to the internal/config package.
//
// Business-rule validators from the module schema (duplicate_ip_per_interface,
// interface_exists, valid_netmask, ip_not_in_reserved_range) are run as
// pluggable functions before any mutation.
type IPAddressModule struct {
	mu     sync.RWMutex
	path   string
	title  string
	schema *config.ModuleSchema

	entries      map[string]*config.Entry // entry ID -> entry
	index        int
	ifaceChecker core.InterfaceChecker
	validators   validatorRegistry
}

// New creates a new IPAddressModule with the given schema.
// The ifaceChecker parameter provides interface name validation; pass nil
// to skip interface-exists checks (not recommended for production).
func New(schema *config.ModuleSchema, ifaceChecker core.InterfaceChecker) (*IPAddressModule, error) {
	if schema == nil {
		return nil, fmt.Errorf("ip_address: schema is required")
	}
	return &IPAddressModule{
		path:         schema.Path,
		title:        schema.Title,
		schema:       schema,
		entries:      make(map[string]*config.Entry),
		ifaceChecker: ifaceChecker,
		validators:   builtinValidators(),
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

// Path returns the full absolute path "/ip/address".
func (m *IPAddressModule) Path() string { return m.path }

// Type returns core.NodeTypeList.
func (m *IPAddressModule) Type() core.NodeType { return core.NodeTypeList }

// Title returns the human-readable display name.
func (m *IPAddressModule) Title() string { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface
//
// A settings directory (list node) may contain sub-directories in theory,
// but /ip/address is a leaf list. Children() returns an empty map and
// AddChild/RemoveChild return errors.
// ---------------------------------------------------------------------------

// Children returns an empty map — /ip/address is a leaf list node.
func (m *IPAddressModule) Children() map[string]core.Node { return nil }

// AddChild returns an error — a list node cannot contain children.
func (m *IPAddressModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("ip_address: list node %q cannot accept child %q", m.path, name)
}

// RemoveChild returns an error — a list node has no children to remove.
func (m *IPAddressModule) RemoveChild(name string) error {
	return fmt.Errorf("ip_address: list node %q has no child %q", m.path, name)
}

// Child returns (nil, false) — /ip/address has no child nodes.
func (m *IPAddressModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new IP address entry after applying defaults, sanitizing
// properties, and running business-rule validators (duplicate_ip_per_interface,
// interface_exists, valid_netmask, ip_not_in_reserved_range).
//
// The returned Entry is a snapshot of the stored entry.
func (m *IPAddressModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Sanitize and coerce property values against the schema
	sanitized, err := m.coerceProperties(props)
	if err != nil {
		return nil, err
	}

	// 2. Validate required properties
	address, hasAddr := sanitized["address"]
	if !hasAddr || address == "" {
		return nil, fmt.Errorf("ip_address: required property %q is missing", "address")
	}
	iface, hasIface := sanitized["interface"]
	if !hasIface || iface == "" {
		return nil, fmt.Errorf("ip_address: required property %q is missing", "interface")
	}

	// 3. Run business-rule validators from the "add" action
	action, ok := m.schema.GetAction("add")
	if ok && len(action.Validators) > 0 {
		entries := m.entryList()
		if err := runValidators(action.Validators, sanitized, entries, m.ifaceChecker, m.validators); err != nil {
			return nil, err
		}
	}

	// 4. Build the entry with defaults
	entry := config.NewEntry(uuid.New().String(), m.index)
	m.index++

	// Apply schema defaults
	for name, defaultVal := range m.schema.Defaults {
		if name == "disabled" || name == "vrf" {
			continue
		}
		if _, exists := m.schema.Schema[name]; !exists {
			continue
		}
		entry.Properties[name] = &config.PropertyValue{
			Name:  name,
			Value: defaultVal,
		}
	}

	// Apply provided (sanitized) properties
	for name, val := range sanitized {
		propDef, exists := m.schema.GetProperty(name)
		if !exists {
			continue
		}
		entry.Properties[name] = &config.PropertyValue{
			Name:     name,
			Type:     string(propDef.Type),
			Value:    val,
			ReadOnly: propDef.ReadOnly,
			Required: propDef.Required,
		}
	}

	// Apply the "add" action's flags_set
	if action, ok := m.schema.GetAction("add"); ok {
		for _, flagName := range action.FlagsSet {
			switch flagName {
			case "disabled":
				entry.Disabled = true
			}
		}
	}

	// 5. Store the entry
	m.entries[entry.ID] = entry

	return &entryAdapter{entry: entry.Clone()}, nil
}

// Set updates properties of an existing entry identified by its ID.
// Property values are sanitized and coerced against the schema, then
// business-rule validators are run before applying changes.
// Returns an error if the entry does not exist or a property is read-only.
func (m *IPAddressModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("ip_address: entry %q not found", id)
	}

	// 1. Sanitize and coerce property values
	sanitized, err := m.coerceProperties(props)
	if err != nil {
		return err
	}

	// 2. Run business-rule validators from the "set" action
	//    Merge sanitized values with existing entry values so validators
	//    like duplicate_ip_per_interface can check the full picture.
	merged := make(map[string]interface{})
	for k, pv := range entry.Properties {
		merged[k] = pv.Value
	}
	for k, v := range sanitized {
		merged[k] = v
	}

	action, ok := m.schema.GetAction("set")
	if ok && len(action.Validators) > 0 {
		entries := m.entryListWithExclusion(id)
		if err := runValidators(action.Validators, merged, entries, m.ifaceChecker, m.validators); err != nil {
			return err
		}
	}

	// 3. Apply changes
	for name, rawVal := range props {
		propDef, exists := m.schema.GetProperty(name)
		if !exists {
			return fmt.Errorf("ip_address: property %q is not defined in schema", name)
		}
		if propDef.ReadOnly {
			return fmt.Errorf("ip_address: property %q is read-only", name)
		}
		coerced, err := config.CoercePropertyValue(rawVal, propDef.Type)
		if err != nil {
			return fmt.Errorf("ip_address: property %q: %w", name, err)
		}

		if pv, ok := entry.Properties[name]; ok {
			pv.Value = coerced
		} else {
			entry.Properties[name] = &config.PropertyValue{
				Name:     name,
				Type:     string(propDef.Type),
				Value:    coerced,
				ReadOnly: propDef.ReadOnly,
				Required: propDef.Required,
			}
		}
	}

	return nil
}

// Remove deletes the entry with the given ID.
// Returns an error if the entry does not exist or is dynamic.
func (m *IPAddressModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("ip_address: entry %q not found", id)
	}
	if entry.Dynamic {
		return fmt.Errorf("ip_address: cannot remove dynamic entry %q", id)
	}

	delete(m.entries, id)
	return nil
}

// List returns all entries ordered by their index.
func (m *IPAddressModule) List() []core.Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.entryList()
}

// Get returns the entry with the given ID, or false if not found.
func (m *IPAddressModule) Get(id string) (core.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[id]
	if !exists {
		return nil, false
	}
	return &entryAdapter{entry: entry.Clone()}, true
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// coerceProperties sanitizes and coerces raw property values against the
// module schema, returning the sanitized map or an error.
func (m *IPAddressModule) coerceProperties(props map[string]interface{}) (map[string]interface{}, error) {
	sanitized := make(map[string]interface{}, len(props))
	for name, rawVal := range props {
		propDef, exists := m.schema.GetProperty(name)
		if !exists {
			return nil, fmt.Errorf("ip_address: property %q is not defined in schema", name)
		}
		coerced, err := config.CoercePropertyValue(rawVal, propDef.Type)
		if err != nil {
			return nil, fmt.Errorf("ip_address: property %q: %w", name, err)
		}
		sanitized[name] = coerced
	}
	return sanitized, nil
}

// entryList returns all entries as a sorted []core.Entry slice.
// Caller must hold at least a read lock.
func (m *IPAddressModule) entryList() []core.Entry {
	result := make([]core.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, &entryAdapter{entry: e.Clone()})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Index() < result[j].Index()
	})
	return result
}

// entryListWithExclusion returns all entries as a sorted []core.Entry slice,
// excluding the entry with the given ID. Used during Set to avoid false
// duplicate-detection when updating the entry itself.
// Caller must hold at least a read lock.
func (m *IPAddressModule) entryListWithExclusion(excludeID string) []core.Entry {
	result := make([]core.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		if e.ID == excludeID {
			continue
		}
		result = append(result, &entryAdapter{entry: e.Clone()})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Index() < result[j].Index()
	})
	return result
}
