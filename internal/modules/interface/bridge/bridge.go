// Package bridge implements the /interface bridge settings directory for RouterOS v7.
// It provides creating, removing, and listing bridge interfaces, as well as
// MAC learning and forwarding functionality.
package bridge

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/topology"
)

// ---------------------------------------------------------------------------
// MAC Table Entry
// ---------------------------------------------------------------------------

// macEntry represents a learned MAC address associated with a bridge port.
type macEntry struct {
	mac       string
	portName  string
	learnedAt time.Time
}

// ---------------------------------------------------------------------------
// Bridge Entry — internal representation
// ---------------------------------------------------------------------------

// bridgeEntry stores all properties for a single bridge interface.
type bridgeEntry struct {
	id            string
	index         int
	name          string
	mtu           int
	ageingTime    time.Duration
	disabled      bool
	protocolMode  string // "none", "stp", "rstp", "mstp"
	vlanFiltering bool
	macAddress    string
	running       bool

	// MAC learning / forwarding table: mac -> macEntry
	forwardingTable map[string]*macEntry
	// Port members: port name -> true
	ports map[string]bool
	mu    sync.RWMutex
}

// newBridgeEntry creates a new bridge entry with defaults.
func newBridgeEntry(id string, index int, name string) *bridgeEntry {
	return &bridgeEntry{
		id:              id,
		index:           index,
		name:            name,
		mtu:             1500,
		ageingTime:      5 * time.Minute,
		disabled:        false,
		protocolMode:    "none",
		vlanFiltering:   false,
		macAddress:      generateBridgeMAC(index),
		running:         true,
		forwardingTable: make(map[string]*macEntry),
		ports:           make(map[string]bool),
	}
}

// generateBridgeMAC produces a deterministic MAC address for a bridge.
func generateBridgeMAC(index int) string {
	// Use a different OUI prefix (02:00:00) to distinguish from physical interfaces
	octet1 := (index / 256) % 256
	octet2 := index % 256
	return fmt.Sprintf("02:00:00:00:%02X:%02X", octet1, octet2)
}

// ---------------------------------------------------------------------------
// entryAdapter — wraps bridgeEntry to satisfy core.Entry
// ---------------------------------------------------------------------------

type entryAdapter struct {
	entry *bridgeEntry
}

func (a *entryAdapter) ID() string     { return a.entry.id }
func (a *entryAdapter) Index() int     { return a.entry.index }
func (a *entryAdapter) Disabled() bool { return a.entry.disabled }
func (a *entryAdapter) Dynamic() bool  { return false }
func (a *entryAdapter) Invalid() bool  { return false }
func (a *entryAdapter) Slave() bool    { return false }

func (a *entryAdapter) Property(name string) (interface{}, bool) {
	switch name {
	case "name":
		return a.entry.name, true
	case "mtu":
		return a.entry.mtu, true
	case "ageing-time":
		return formatDuration(a.entry.ageingTime), true
	case "disabled":
		return a.entry.disabled, true
	case "protocol-mode":
		return a.entry.protocolMode, true
	case "vlan-filtering":
		return a.entry.vlanFiltering, true
	case "mac-address":
		return a.entry.macAddress, true
	case "running":
		return a.entry.running, true
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
		if i < 68 || i > 1500 {
			return fmt.Errorf("mtu must be between 68 and 1500")
		}
		a.entry.mtu = i
	case "ageing-time":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("ageing-time must be a string")
		}
		d, err := parseDuration(s)
		if err != nil {
			return fmt.Errorf("ageing-time: %w", err)
		}
		a.entry.ageingTime = d
	case "disabled":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("disabled: %w", err)
		}
		a.entry.disabled = b
		a.entry.running = !b && len(a.entry.ports) > 0
	case "protocol-mode":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("protocol-mode must be a string")
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "none", "stp", "rstp", "mstp":
			a.entry.protocolMode = s
		default:
			return fmt.Errorf("protocol-mode must be one of: none, stp, rstp, mstp")
		}
	case "vlan-filtering":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("vlan-filtering: %w", err)
		}
		a.entry.vlanFiltering = b
	default:
		return fmt.Errorf("property %q is read-only or does not exist", name)
	}
	return nil
}

func (a *entryAdapter) Properties() map[string]interface{} {
	return map[string]interface{}{
		"name":           a.entry.name,
		"mtu":            a.entry.mtu,
		"ageing-time":    formatDuration(a.entry.ageingTime),
		"disabled":       a.entry.disabled,
		"protocol-mode":  a.entry.protocolMode,
		"vlan-filtering": a.entry.vlanFiltering,
		"mac-address":    a.entry.macAddress,
		"running":        a.entry.running,
	}
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"disabled": a.entry.disabled,
		"running":  a.entry.running,
		"dynamic":  false,
	}
}

// Compile-time checks
var _ core.Entry = (*entryAdapter)(nil)
var _ core.SettingsDirectory = (*BridgeModule)(nil)
var _ topology.BridgeHandler = (*BridgeModule)(nil)

// ---------------------------------------------------------------------------
// BridgeModule — implements core.SettingsDirectory and topology.BridgeHandler
// ---------------------------------------------------------------------------

// InterfaceChecker provides interface name validation and listing.
type InterfaceChecker interface {
	InterfaceExists(name string) bool
	ListInterfaces() []string
}

// BridgeModule provides full CRUD for /interface bridge entries.
// It maintains an in-memory map of bridges and their forwarding tables.
type BridgeModule struct {
	mu      sync.RWMutex
	path    string
	title   string
	schema  interface{}
	entries map[string]*bridgeEntry // id -> entry
	byName  map[string]string       // name -> id
	index   int
}

// New creates a new BridgeModule.
func New(schema interface{}) (*BridgeModule, error) {
	return &BridgeModule{
		path:    "/interface bridge",
		title:   "Bridge",
		schema:  schema,
		entries: make(map[string]*bridgeEntry),
		byName:  make(map[string]string),
	}, nil
}

// ---------------------------------------------------------------------------
// Public API for bridge operations
// ---------------------------------------------------------------------------

// AddBridge creates a new bridge with the given name.
func (m *BridgeModule) AddBridge(name string) (core.Entry, error) {
	return m.Add(map[string]interface{}{
		"name": name,
	})
}

// RemoveBridge deletes a bridge by name.
func (m *BridgeModule) RemoveBridge(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, exists := m.byName[name]
	if !exists {
		return fmt.Errorf("bridge: bridge %q not found", name)
	}
	delete(m.entries, id)
	delete(m.byName, name)
	return nil
}

// GetBridge returns the bridge entry by name, or nil if not found.
func (m *BridgeModule) GetBridge(name string) *bridgeEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, exists := m.byName[name]
	if !exists {
		return nil
	}
	return m.entries[id]
}

// BridgeExists checks if a bridge with the given name exists.
func (m *BridgeModule) BridgeExists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.byName[name]
	return ok
}

// AddPort adds a port to a bridge.
func (m *BridgeModule) AddPort(bridgeName, portName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, exists := m.byName[bridgeName]
	if !exists {
		return fmt.Errorf("bridge: bridge %q not found", bridgeName)
	}
	entry := m.entries[id]
	entry.mu.Lock()
	entry.ports[portName] = true
	entry.running = !entry.disabled && len(entry.ports) > 0
	entry.mu.Unlock()
	return nil
}

// RemovePort removes a port from a bridge.
func (m *BridgeModule) RemovePort(bridgeName, portName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, exists := m.byName[bridgeName]
	if !exists {
		return fmt.Errorf("bridge: bridge %q not found", bridgeName)
	}
	entry := m.entries[id]
	entry.mu.Lock()
	delete(entry.ports, portName)

	// Remove all MAC entries associated with this port
	for mac, macEntry := range entry.forwardingTable {
		if macEntry.portName == portName {
			delete(entry.forwardingTable, mac)
		}
	}
	entry.running = !entry.disabled && len(entry.ports) > 0
	entry.mu.Unlock()
	return nil
}

// HasPort checks if a port is a member of any bridge.
func (m *BridgeModule) HasPort(portName string) (bridgeName string, found bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, entry := range m.entries {
		entry.mu.RLock()
		if entry.ports[portName] {
			bridgeName = entry.name
			found = true
			entry.mu.RUnlock()
			return
		}
		entry.mu.RUnlock()
	}
	return "", false
}

// GetBridgeByPort returns the bridge name that contains the given port.
func (m *BridgeModule) GetBridgeByPort(portName string) (string, bool) {
	return m.HasPort(portName)
}

// ---------------------------------------------------------------------------
// MAC Learning and Forwarding
// ---------------------------------------------------------------------------

// AddMAC adds or updates a MAC address in the forwarding table.
func (m *BridgeModule) AddMAC(bridgeName, mac, portName string) {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return
	}

	m.entries[id].mu.Lock()
	defer m.entries[id].mu.Unlock()

	// Normalize MAC
	mac = normalizeMAC(mac)

	// Update existing or add new entry
	m.entries[id].forwardingTable[mac] = &macEntry{
		mac:       mac,
		portName:  portName,
		learnedAt: time.Now(),
	}
}

// RemoveMAC removes a MAC address from the forwarding table.
func (m *BridgeModule) RemoveMAC(bridgeName, mac string) {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return
	}

	m.entries[id].mu.Lock()
	defer m.entries[id].mu.Unlock()

	mac = normalizeMAC(mac)
	delete(m.entries[id].forwardingTable, mac)
}

// LookupMAC looks up a MAC in the forwarding table.
// Returns the port name and true if found.
func (m *BridgeModule) LookupMAC(bridgeName, mac string) (string, bool) {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return "", false
	}

	m.entries[id].mu.RLock()
	defer m.entries[id].mu.RUnlock()

	mac = normalizeMAC(mac)
	entry, found := m.entries[id].forwardingTable[mac]
	if !found {
		return "", false
	}
	return entry.portName, true
}

// AgeMACs removes aged-out MAC entries from the forwarding table.
func (m *BridgeModule) AgeMACs(bridgeName string) int {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return 0
	}

	m.entries[id].mu.Lock()
	defer m.entries[id].mu.Unlock()

	now := time.Now()
	aged := 0
	for mac, entry := range m.entries[id].forwardingTable {
		if now.Sub(entry.learnedAt) > m.entries[id].ageingTime {
			delete(m.entries[id].forwardingTable, mac)
			aged++
		}
	}
	return aged
}

// AgeAllMACs ages all bridges' forwarding tables. Returns total aged entries.
func (m *BridgeModule) AgeAllMACs() int {
	m.mu.RLock()
	names := make([]string, 0, len(m.byName))
	for name := range m.byName {
		names = append(names, name)
	}
	m.mu.RUnlock()

	total := 0
	for _, name := range names {
		total += m.AgeMACs(name)
	}
	return total
}

// GetForwardingTable returns a copy of the forwarding table for a bridge.
func (m *BridgeModule) GetForwardingTable(bridgeName string) map[string]string {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return nil
	}

	m.entries[id].mu.RLock()
	defer m.entries[id].mu.RUnlock()

	result := make(map[string]string, len(m.entries[id].forwardingTable))
	for mac, entry := range m.entries[id].forwardingTable {
		result[mac] = entry.portName
	}
	return result
}

// GetBridgePorts returns a list of port names for a bridge.
func (m *BridgeModule) GetBridgePorts(bridgeName string) []string {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return nil
	}

	m.entries[id].mu.RLock()
	defer m.entries[id].mu.RUnlock()

	ports := make([]string, 0, len(m.entries[id].ports))
	for p := range m.entries[id].ports {
		ports = append(ports, p)
	}
	return ports
}

// ---------------------------------------------------------------------------
// BridgeHandler interface implementation (for topology packet forwarding)
// ---------------------------------------------------------------------------

// HandlePacket processes a packet arriving on a bridged interface.
// It performs MAC learning and forwarding/flooding.
// Returns the list of outIface names to forward to, excluding the incoming port.
func (m *BridgeModule) HandlePacket(bridgeName string, inPort string, packet topology.Packet) []topology.ForwardAction {
	m.mu.RLock()
	id, exists := m.byName[bridgeName]
	m.mu.RUnlock()
	if !exists {
		return nil
	}

	entry := m.entries[id]
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// MAC Learning: associate source MAC with incoming port
	srcMAC := packet.Eth.SrcMAC
	if srcMAC != "" {
		srcMAC = normalizeMAC(srcMAC)
		entry.forwardingTable[srcMAC] = &macEntry{
			mac:       srcMAC,
			portName:  inPort,
			learnedAt: time.Now(),
		}
	}

	dstMAC := packet.Eth.DstMAC
	if dstMAC != "" {
		dstMAC = normalizeMAC(dstMAC)
	}

	// Check if destination is broadcast or unknown unicast
	isFlood := false
	if dstMAC == "" || dstMAC == topology.BroadcastMAC {
		isFlood = true
	} else if entry.forwardingTable[dstMAC] == nil {
		// Unknown unicast: flood
		isFlood = true
	}

	if isFlood {
		// Flood to all ports except the incoming port
		var actions []topology.ForwardAction
		for port := range entry.ports {
			if port != inPort {
				actions = append(actions, topology.ForwardAction{OutIface: port})
			}
		}
		return actions
	}

	// Known unicast: forward to specific port
	macEntry := entry.forwardingTable[dstMAC]
	if macEntry.portName == inPort {
		// Destination is on the same port as source; drop (switch optimization)
		return nil
	}

	return []topology.ForwardAction{
		{OutIface: macEntry.portName},
	}
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

func (m *BridgeModule) Path() string        { return m.path }
func (m *BridgeModule) Type() core.NodeType { return core.NodeTypeList }
func (m *BridgeModule) Title() string       { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

func (m *BridgeModule) Children() map[string]core.Node { return nil }
func (m *BridgeModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("bridge: list node %q cannot accept child %q", m.path, name)
}
func (m *BridgeModule) RemoveChild(name string) error {
	return fmt.Errorf("bridge: list node %q has no child %q", m.path, name)
}
func (m *BridgeModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new bridge entry.
func (m *BridgeModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Extract and validate name
	nameRaw, hasName := props["name"]
	if !hasName {
		return nil, fmt.Errorf("bridge: required property %q is missing", "name")
	}
	name, ok := nameRaw.(string)
	if !ok || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("bridge: property %q must be a non-empty string", "name")
	}
	name = strings.TrimSpace(name)

	// 2. Validate unique name
	if _, exists := m.byName[name]; exists {
		return nil, fmt.Errorf("bridge: bridge %q already exists", name)
	}

	// 3. Build entry with defaults
	entry := newBridgeEntry(strconv.Itoa(m.index), m.index, name)
	m.index++

	// 4. Apply overrides from props
	if err := applyBridgeProps(entry, props); err != nil {
		return nil, err
	}

	// 5. Store
	m.entries[entry.id] = entry
	m.byName[entry.name] = entry.id

	return &entryAdapter{entry: entry}, nil
}

// Set updates properties of an existing bridge.
func (m *BridgeModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("bridge: entry %q not found", id)
	}

	// If name is being changed, validate uniqueness
	if nameRaw, hasName := props["name"]; hasName {
		newName, ok := nameRaw.(string)
		if !ok || strings.TrimSpace(newName) == "" {
			return fmt.Errorf("bridge: property %q must be a non-empty string", "name")
		}
		newName = strings.TrimSpace(newName)
		if newName != entry.name {
			if existingID, taken := m.byName[newName]; taken && existingID != id {
				return fmt.Errorf("bridge: bridge %q already exists", newName)
			}
			delete(m.byName, entry.name)
			entry.name = newName
			m.byName[newName] = id
		}
		delete(props, "name")
	}

	if err := applyBridgeProps(entry, props); err != nil {
		return err
	}

	return nil
}

// Remove deletes a bridge entry.
func (m *BridgeModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("bridge: entry %q not found", id)
	}

	delete(m.entries, id)
	delete(m.byName, entry.name)
	return nil
}

// List returns all entries sorted by index.
func (m *BridgeModule) List() []core.Entry {
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
func (m *BridgeModule) Get(id string) (core.Entry, bool) {
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

func applyBridgeProps(entry *bridgeEntry, props map[string]interface{}) error {
	for name, rawVal := range props {
		switch name {
		case "mtu":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("bridge: mtu: %w", err)
			}
			if i < 68 || i > 1500 {
				return fmt.Errorf("bridge: mtu must be between 68 and 1500")
			}
			entry.mtu = i
		case "ageing-time":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("bridge: ageing-time must be a string")
			}
			d, err := parseDuration(s)
			if err != nil {
				return fmt.Errorf("bridge: ageing-time: %w", err)
			}
			entry.ageingTime = d
		case "disabled":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("bridge: disabled: %w", err)
			}
			entry.disabled = b
			entry.running = !b && len(entry.ports) > 0
		case "protocol-mode":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("bridge: protocol-mode must be a string")
			}
			s = strings.ToLower(strings.TrimSpace(s))
			switch s {
			case "none", "stp", "rstp", "mstp":
				entry.protocolMode = s
			default:
				return fmt.Errorf("bridge: protocol-mode must be one of: none, stp, rstp, mstp")
			}
		case "vlan-filtering":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("bridge: vlan-filtering: %w", err)
			}
			entry.vlanFiltering = b
		case "name":
			// handled by caller
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Duration parsing helpers
// ---------------------------------------------------------------------------

// durationRegex matches duration strings like "5m", "10s", "1h30m", etc.
var durationRegex = regexp.MustCompile(`^(\d+)([smhd])$`)

// parseDuration parses a RouterOS-style duration string into time.Duration.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)

	// Try Go's native parsing first (supports "5m", "10s", etc.)
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// RouterOS uses "1d" for 1 day, which Go doesn't support
	matches := durationRegex.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %q (use e.g., 5m, 10s, 1h, 1d)", s)
	}

	val, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "s":
		return time.Duration(val) * time.Second, nil
	case "m":
		return time.Duration(val) * time.Minute, nil
	case "h":
		return time.Duration(val) * time.Hour, nil
	case "d":
		return time.Duration(val) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %q", unit)
	}
}

// formatDuration formats a time.Duration as a RouterOS-compatible string.
func formatDuration(d time.Duration) string {
	// Simple formatting: show in most readable unit
	if d >= 24*time.Hour {
		days := d / (24 * time.Hour)
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		hours := d / time.Hour
		return fmt.Sprintf("%dh", hours)
	}
	if d >= time.Minute {
		mins := d / time.Minute
		return fmt.Sprintf("%dm", mins)
	}
	secs := d / time.Second
	return fmt.Sprintf("%ds", secs)
}

// ---------------------------------------------------------------------------
// MAC normalization
// ---------------------------------------------------------------------------

// macNormalizeRegex matches standard MAC address hex digits.
var macNormalizeRegex = regexp.MustCompile(`[^0-9A-Fa-f]`)

// normalizeMAC converts any MAC format to "xx:xx:xx:xx:xx:xx" uppercase.
func normalizeMAC(mac string) string {
	// Remove all non-hex characters
	cleaned := macNormalizeRegex.ReplaceAllString(mac, "")
	if len(cleaned) != 12 {
		return strings.ToUpper(mac) // return as-is if not standard length
	}

	// Format as xx:xx:xx:xx:xx:xx
	var parts []string
	for i := 0; i < 12; i += 2 {
		parts = append(parts, cleaned[i:i+2])
	}
	return strings.ToUpper(strings.Join(parts, ":"))
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
