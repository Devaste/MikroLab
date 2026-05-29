// Package ip_arp implements the /ip/arp settings directory for ARP table management.
// It provides CRUD operations on static ARP entries and validates against the
// interface module for interface existence checks.
package ip_arp

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Devaste/MikroLab/internal/core"
)

// ---------------------------------------------------------------------------
// arpEntry — internal representation of a single ARP entry
// ---------------------------------------------------------------------------

// arpEntry stores all properties for a single ARP entry.
type arpEntry struct {
	id        string
	index     int
	address   string
	macAddr   string
	iface     string
	published bool
	disabled  bool
	dynamic   bool
	status    string // "complete" for static entries
}

// ---------------------------------------------------------------------------
// entryAdapter — wraps arpEntry to satisfy core.Entry
// ---------------------------------------------------------------------------

type entryAdapter struct {
	entry *arpEntry
}

func (a *entryAdapter) ID() string     { return a.entry.id }
func (a *entryAdapter) Index() int     { return a.entry.index }
func (a *entryAdapter) Disabled() bool { return a.entry.disabled }
func (a *entryAdapter) Dynamic() bool  { return a.entry.dynamic }
func (a *entryAdapter) Invalid() bool  { return false }
func (a *entryAdapter) Slave() bool    { return false }

func (a *entryAdapter) Property(name string) (interface{}, bool) {
	switch name {
	case "address":
		return a.entry.address, true
	case "mac-address":
		return a.entry.macAddr, true
	case "interface":
		return a.entry.iface, true
	case "published":
		return a.entry.published, true
	case "disabled":
		return a.entry.disabled, true
	case "dynamic":
		return a.entry.dynamic, true
	case "status":
		return a.entry.status, true
	case "dhcp":
		return false, true
	default:
		return nil, false
	}
}

func (a *entryAdapter) SetProperty(name string, value interface{}) error {
	switch name {
	case "mac-address":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("mac-address must be a string")
		}
		if !isValidMAC(s) {
			return fmt.Errorf("invalid MAC address format: %q", s)
		}
		a.entry.macAddr = normalizeMAC(s)
	case "published":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("published: %w", err)
		}
		a.entry.published = b
	case "disabled":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("disabled: %w", err)
		}
		a.entry.disabled = b
	case "interface":
		// interface change is not allowed to keep uniqueness simple
		return fmt.Errorf("interface is read-only after creation")
	case "address":
		return fmt.Errorf("address is read-only after creation")
	default:
		return fmt.Errorf("property %q is read-only or does not exist", name)
	}
	return nil
}

func (a *entryAdapter) Properties() map[string]interface{} {
	return map[string]interface{}{
		"address":     a.entry.address,
		"mac-address": a.entry.macAddr,
		"interface":   a.entry.iface,
		"published":   a.entry.published,
		"disabled":    a.entry.disabled,
		"dynamic":     a.entry.dynamic,
		"status":      a.entry.status,
	}
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"disabled":  a.entry.disabled,
		"invalid":   a.entry.dynamic && a.entry.iface == "", // not used for static
		"dhcp":      false,                                  // for MVP, ignore DHCP flag
		"dynamic":   a.entry.dynamic,
		"published": a.entry.published,
		"complete":  a.entry.status == "complete",
	}
}

// Compile-time check
var _ core.Entry = (*entryAdapter)(nil)

// ---------------------------------------------------------------------------
// InterfaceChecker — subset of core.InterfaceChecker for ARP validation
// ---------------------------------------------------------------------------

// InterfaceChecker provides interface name validation for ARP entries.
type InterfaceChecker interface {
	InterfaceExists(name string) bool
	ListInterfaces() []string
}

// ---------------------------------------------------------------------------
// ArpModule — implements core.SettingsDirectory
// ---------------------------------------------------------------------------

// ArpModule provides full CRUD for /ip/arp entries.
// It maintains an in-memory map of static ARP entries and validates
// interface references against the interface module.
type ArpModule struct {
	mu    sync.RWMutex
	path  string
	title string

	entries      map[string]*arpEntry // id -> entry
	byIPIface    map[string]string    // "ip|interface" -> id (for uniqueness check)
	index        int
	ifaceChecker InterfaceChecker
}

// New creates a new ArpModule with the given schema and interface checker.
func New(path string, title string, ifaceChecker InterfaceChecker) (*ArpModule, error) {
	if path == "" {
		return nil, fmt.Errorf("arp: path is required")
	}
	if ifaceChecker == nil {
		return nil, fmt.Errorf("arp: interface checker is required")
	}
	return &ArpModule{
		path:         path,
		title:        title,
		entries:      make(map[string]*arpEntry),
		byIPIface:    make(map[string]string),
		ifaceChecker: ifaceChecker,
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

func (m *ArpModule) Path() string        { return m.path }
func (m *ArpModule) Type() core.NodeType { return core.NodeTypeList }
func (m *ArpModule) Title() string       { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

func (m *ArpModule) Children() map[string]core.Node { return nil }
func (m *ArpModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("arp: list node %q cannot accept child %q", m.path, name)
}
func (m *ArpModule) RemoveChild(name string) error {
	return fmt.Errorf("arp: list node %q has no child %q", m.path, name)
}
func (m *ArpModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new ARP entry after validation.
// Required properties: address, mac-address, interface.
// Optional properties: published, disabled.
func (m *ArpModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Extract and validate required properties
	addressRaw, hasAddr := props["address"]
	if !hasAddr {
		return nil, fmt.Errorf("arp: required property %q is missing", "address")
	}
	address, ok := addressRaw.(string)
	if !ok || strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("arp: property %q must be a non-empty string", "address")
	}
	address = strings.TrimSpace(address)

	macRaw, hasMAC := props["mac-address"]
	if !hasMAC {
		return nil, fmt.Errorf("arp: required property %q is missing", "mac-address")
	}
	macAddr, ok := macRaw.(string)
	if !ok || strings.TrimSpace(macAddr) == "" {
		return nil, fmt.Errorf("arp: property %q must be a non-empty string", "mac-address")
	}
	macAddr = strings.TrimSpace(macAddr)

	ifaceRaw, hasIface := props["interface"]
	if !hasIface {
		return nil, fmt.Errorf("arp: required property %q is missing", "interface")
	}
	iface, ok := ifaceRaw.(string)
	if !ok || strings.TrimSpace(iface) == "" {
		return nil, fmt.Errorf("arp: property %q must be a non-empty string", "interface")
	}
	iface = strings.TrimSpace(iface)

	// 2. Validate IP address format
	if net.ParseIP(address) == nil {
		return nil, fmt.Errorf("arp: invalid IP address: %q", address)
	}

	// 3. Validate MAC address format
	if !isValidMAC(macAddr) {
		return nil, fmt.Errorf("arp: invalid MAC address: %q", macAddr)
	}
	normalizedMAC := normalizeMAC(macAddr)

	// 4. Validate interface exists
	if !m.ifaceChecker.InterfaceExists(iface) {
		return nil, fmt.Errorf("arp: interface %q does not exist", iface)
	}

	// 5. Check uniqueness: same IP cannot be mapped to two MACs on same interface
	key := ipIfaceKey(address, iface)
	if existingID, exists := m.byIPIface[key]; exists {
		if existingEntry, found := m.entries[existingID]; found {
			return nil, fmt.Errorf("arp: duplicate IP %q on interface %q (existing MAC: %s)",
				address, iface, existingEntry.macAddr)
		}
	}

	// 6. Build entry with defaults
	entry := &arpEntry{
		id:        strconv.Itoa(m.index),
		index:     m.index,
		address:   address,
		macAddr:   normalizedMAC,
		iface:     iface,
		published: false,
		disabled:  false,
		dynamic:   false,
		status:    "complete",
	}
	m.index++

	// 7. Apply overrides from props
	if err := applyARPProps(entry, props); err != nil {
		return nil, err
	}

	// 8. Store
	m.entries[entry.id] = entry
	m.byIPIface[key] = entry.id

	return &entryAdapter{entry: entry}, nil
}

// Set updates properties of an existing ARP entry.
// Only mac-address, published, and disabled can be changed.
func (m *ArpModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("arp: entry %q not found", id)
	}
	if entry.dynamic {
		return fmt.Errorf("arp: cannot modify dynamic entry %q", id)
	}

	// Apply props
	if err := applyARPProps(entry, props); err != nil {
		return err
	}

	return nil
}

// Remove deletes an ARP entry.
func (m *ArpModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("arp: entry %q not found", id)
	}
	if entry.dynamic {
		return fmt.Errorf("arp: cannot remove dynamic entry %q", id)
	}

	// Remove from uniqueness map
	key := ipIfaceKey(entry.address, entry.iface)
	delete(m.byIPIface, key)

	delete(m.entries, id)
	return nil
}

// List returns all entries sorted by index.
func (m *ArpModule) List() []core.Entry {
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
func (m *ArpModule) Get(id string) (core.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[id]
	if !exists {
		return nil, false
	}
	return &entryAdapter{entry: entry}, true
}

// ---------------------------------------------------------------------------
// Public helper: Resolve
// ---------------------------------------------------------------------------

// Resolve searches for a static ARP entry with the matching IP and interface.
// If ifaceName is empty, it returns the first matching entry regardless of interface.
// Returns the MAC address and true if found, or empty string and false if not.
// For MVP, this only checks static entries.
func (m *ArpModule) Resolve(ip string, ifaceName string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ip = strings.TrimSpace(ip)
	ifaceName = strings.TrimSpace(ifaceName)

	for _, entry := range m.entries {
		if entry.disabled {
			continue
		}
		if entry.address != ip {
			continue
		}
		if ifaceName != "" && entry.iface != ifaceName {
			continue
		}
		return entry.macAddr, true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// applyARPProps applies writable properties to an arpEntry.
func applyARPProps(entry *arpEntry, props map[string]interface{}) error {
	for name, rawVal := range props {
		switch name {
		case "mac-address":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("arp: mac-address must be a string")
			}
			s = strings.TrimSpace(s)
			if !isValidMAC(s) {
				return fmt.Errorf("arp: invalid MAC address: %q", s)
			}
			entry.macAddr = normalizeMAC(s)
		case "published":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("arp: published: %w", err)
			}
			entry.published = b
		case "disabled":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("arp: disabled: %w", err)
			}
			entry.disabled = b
		case "address":
			// address is not allowed to change (keep uniqueness simple)
			// silently ignore for backward compatibility with schema
		case "interface":
			// interface is not allowed to change
			// silently ignore for backward compatibility with schema
		}
	}
	return nil
}

// ipIfaceKey generates a composite key for uniqueness checks.
func ipIfaceKey(address, iface string) string {
	return address + "|" + iface
}

// ---------------------------------------------------------------------------
// MAC address validation
// ---------------------------------------------------------------------------

// macRegex matches standard MAC address formats:
// - xx:xx:xx:xx:xx:xx
// - xx-xx-xx-xx-xx-xx
// - xxxx.xxxx.xxxx
var macRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
var macCiscoRegex = regexp.MustCompile(`^([0-9A-Fa-f]{4}\.){2}([0-9A-Fa-f]{4})$`)

// isValidMAC checks if the given string is a valid MAC address.
func isValidMAC(mac string) bool {
	mac = strings.TrimSpace(mac)
	if macRegex.MatchString(mac) {
		return true
	}
	if macCiscoRegex.MatchString(mac) {
		return true
	}
	return false
}

// normalizeMAC converts various MAC formats to xx:xx:xx:xx:xx:xx (uppercase).
func normalizeMAC(mac string) string {
	mac = strings.TrimSpace(mac)

	// Handle Cisco format: xxxx.xxxx.xxxx
	if strings.Contains(mac, ".") {
		parts := strings.Split(mac, ".")
		if len(parts) == 3 {
			var result []string
			for _, part := range parts {
				if len(part) == 4 {
					result = append(result, part[:2], part[2:])
				}
			}
			if len(result) == 6 {
				return strings.ToUpper(strings.Join(result, ":"))
			}
		}
	}

	// Handle xx:xx:xx:xx:xx:xx or xx-xx-xx-xx-xx-xx
	separator := ":"
	if strings.Contains(mac, "-") {
		separator = "-"
	}

	parts := strings.Split(mac, separator)
	if len(parts) == 6 {
		for i, p := range parts {
			parts[i] = strings.ToUpper(p)
		}
		return strings.Join(parts, ":")
	}

	return strings.ToUpper(mac)
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
