// Package route implements the /ip/route settings directory.
// It provides full CRUD operations on IPv4 routes and satisfies
// both core.SettingsDirectory and provides public methods for
// other modules (e.g., /ip/address) to manage connected routes.
package route

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
)

// ---------------------------------------------------------------------------
// routeEntry — internal representation of a single route
// ---------------------------------------------------------------------------

// routeEntry stores all properties for a single IP route.
type routeEntry struct {
	id           string
	index        int
	dstAddress   string
	gateway      string
	distance     int
	prefSrc      string
	routingTable string
	disabled     bool
	blackhole    bool
	checkGateway string
	dynamic      bool
	active       bool
	connect      bool // true if this is a connected route
}

// ---------------------------------------------------------------------------
// entryAdapter — wraps routeEntry to satisfy core.Entry
// ---------------------------------------------------------------------------

type entryAdapter struct {
	entry *routeEntry
}

func (a *entryAdapter) ID() string     { return a.entry.id }
func (a *entryAdapter) Index() int     { return a.entry.index }
func (a *entryAdapter) Disabled() bool { return a.entry.disabled }
func (a *entryAdapter) Dynamic() bool  { return a.entry.dynamic }
func (a *entryAdapter) Invalid() bool  { return false }
func (a *entryAdapter) Slave() bool    { return false }

func (a *entryAdapter) Property(name string) (interface{}, bool) {
	switch name {
	case "dst-address":
		return a.entry.dstAddress, true
	case "gateway":
		return a.entry.gateway, true
	case "distance":
		return a.entry.distance, true
	case "pref-src":
		return a.entry.prefSrc, true
	case "routing-table":
		return a.entry.routingTable, true
	case "disabled":
		return a.entry.disabled, true
	case "blackhole":
		return a.entry.blackhole, true
	case "check-gateway":
		return a.entry.checkGateway, true
	case "immediate-gw":
		if a.entry.blackhole {
			return "blackhole", true
		}
		// If gateway is an IP, use it directly; otherwise use empty
		if net.ParseIP(a.entry.gateway) != nil {
			return a.entry.gateway, true
		}
		return "", true
	case "local-address":
		return "", true
	case "dynamic":
		return a.entry.dynamic, true
	case "active":
		return a.entry.active, true
	default:
		return nil, false
	}
}

func (a *entryAdapter) SetProperty(name string, value interface{}) error {
	switch name {
	case "dst-address":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("dst-address must be a string")
		}
		a.entry.dstAddress = s
	case "gateway":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("gateway must be a string")
		}
		a.entry.gateway = s
	case "distance":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("distance: %w", err)
		}
		if i < 1 || i > 255 {
			return fmt.Errorf("distance must be between 1 and 255")
		}
		a.entry.distance = i
	case "pref-src":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("pref-src must be a string")
		}
		a.entry.prefSrc = s
	case "routing-table":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("routing-table must be a string")
		}
		a.entry.routingTable = s
	case "disabled":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("disabled: %w", err)
		}
		a.entry.disabled = b
		if b {
			a.entry.active = false
		} else if !a.entry.blackhole {
			a.entry.active = true
		}
	case "blackhole":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("blackhole: %w", err)
		}
		a.entry.blackhole = b
		if b {
			// For blackhole routes, gateway is ignored but route is active
			a.entry.active = !a.entry.disabled
		}
	case "check-gateway":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("check-gateway must be a string")
		}
		a.entry.checkGateway = s
	default:
		return fmt.Errorf("property %q is read-only or does not exist", name)
	}
	return nil
}

func (a *entryAdapter) Properties() map[string]interface{} {
	return map[string]interface{}{
		"dst-address":   a.entry.dstAddress,
		"gateway":       a.entry.gateway,
		"distance":      a.entry.distance,
		"pref-src":      a.entry.prefSrc,
		"routing-table": a.entry.routingTable,
		"disabled":      a.entry.disabled,
		"blackhole":     a.entry.blackhole,
		"check-gateway": a.entry.checkGateway,
		"immediate-gw":  a.entry.gateway,
		"local-address": "",
		"dynamic":       a.entry.dynamic,
		"active":        a.entry.active,
	}
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"active":   a.entry.active,
		"dynamic":  a.entry.dynamic,
		"disabled": a.entry.disabled,
		"connect":  a.entry.connect,
		"static":   !a.entry.dynamic,
		"invalid":  false,
		"slave":    false,
	}
}

// Compile-time check
var _ core.Entry = (*entryAdapter)(nil)

// ---------------------------------------------------------------------------
// InterfaceChecker — subset of core.InterfaceChecker for gateway validation
// ---------------------------------------------------------------------------

// InterfaceChecker provides interface name validation for route gateway.
type InterfaceChecker interface {
	InterfaceExists(name string) bool
	ListInterfaces() []string
}

// ---------------------------------------------------------------------------
// RouteModule — implements core.SettingsDirectory
// ---------------------------------------------------------------------------

// RouteModule provides full CRUD for /ip/route entries.
type RouteModule struct {
	mu     sync.RWMutex
	path   string
	title  string
	schema *config.ModuleSchema

	entries      map[string]*routeEntry // id -> entry
	index        int
	ifaceChecker InterfaceChecker
	validators   validatorRegistry
}

// New creates a new RouteModule with the given schema and interface checker.
// The ifaceChecker is used to validate gateway references to interface names.
func New(schema *config.ModuleSchema, ifaceChecker InterfaceChecker) (*RouteModule, error) {
	if schema == nil {
		return nil, fmt.Errorf("route: schema is required")
	}
	return &RouteModule{
		path:         schema.Path,
		title:        schema.Title,
		schema:       schema,
		entries:      make(map[string]*routeEntry),
		ifaceChecker: ifaceChecker,
		validators:   builtinValidators(),
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

func (m *RouteModule) Path() string        { return m.path }
func (m *RouteModule) Type() core.NodeType { return core.NodeTypeList }
func (m *RouteModule) Title() string       { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

func (m *RouteModule) Children() map[string]core.Node { return nil }
func (m *RouteModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("route: list node %q cannot accept child %q", m.path, name)
}
func (m *RouteModule) RemoveChild(name string) error {
	return fmt.Errorf("route: list node %q has no child %q", m.path, name)
}
func (m *RouteModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new route entry.
func (m *RouteModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Validate required properties
	dstRaw, hasDst := props["dst-address"]
	if !hasDst {
		return nil, fmt.Errorf("route: required property %q is missing", "dst-address")
	}
	dst, ok := dstRaw.(string)
	if !ok || strings.TrimSpace(dst) == "" {
		return nil, fmt.Errorf("route: property %q must be a non-empty string", "dst-address")
	}
	dst = strings.TrimSpace(dst)

	gwRaw, hasGW := props["gateway"]
	if !hasGW {
		return nil, fmt.Errorf("route: required property %q is missing", "gateway")
	}
	gw, ok := gwRaw.(string)
	if !ok || strings.TrimSpace(gw) == "" {
		return nil, fmt.Errorf("route: property %q must be a non-empty string", "gateway")
	}
	gw = strings.TrimSpace(gw)

	// 2. Determine blackhole status early for gateway validation
	blackhole := false
	if bhRaw, hasBH := props["blackhole"]; hasBH {
		bh, err := toBool(bhRaw)
		if err == nil {
			blackhole = bh
		}
	}

	// 3. Run business-rule validators from the "add" action
	action, ok := m.schema.GetAction("add")
	if ok && len(action.Validators) > 0 {
		entries := m.entryList()
		if err := runValidators(action.Validators, props, entries, m.ifaceChecker, m.validators); err != nil {
			return nil, err
		}
	} else {
		// Fallback: inline validation for backward compatibility
		// when schema does not define validators.
		if err := validateCIDR(dst); err != nil {
			return nil, fmt.Errorf("route: %w", err)
		}
		if !blackhole {
			if err := m.validateGateway(gw); err != nil {
				return nil, fmt.Errorf("route: %w", err)
			}
		}
	}

	entry := &routeEntry{
		id:           strconv.Itoa(m.index),
		index:        m.index,
		dstAddress:   dst,
		gateway:      gw,
		distance:     1,
		prefSrc:      "",
		routingTable: "main",
		disabled:     false,
		blackhole:    blackhole,
		checkGateway: "none",
		dynamic:      false,
		active:       true,
		connect:      false,
	}
	m.index++

	// 4. Apply overrides from props
	if err := applyRouteProps(entry, props); err != nil {
		return nil, err
	}

	// For non-blackhole routes, ensure active is based on disabled state
	if !entry.active && !entry.disabled {
		entry.active = true
	}

	// 5. Store
	m.entries[entry.id] = entry

	return &entryAdapter{entry: entry}, nil
}

// Set updates properties of an existing route.
func (m *RouteModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("route: entry %q not found", id)
	}
	if entry.dynamic {
		return fmt.Errorf("route: cannot modify dynamic entry %q", id)
	}

	// Run business-rule validators from the "set" action
	// Merge sanitized values with existing entry values so validators
	// like valid_gateway can check the full picture.
	merged := make(map[string]interface{})
	merged["dst-address"] = entry.dstAddress
	merged["gateway"] = entry.gateway
	merged["blackhole"] = entry.blackhole
	for k, v := range props {
		merged[k] = v
	}

	action, ok := m.schema.GetAction("set")
	if ok && len(action.Validators) > 0 {
		entries := m.entryList()
		if err := runValidators(action.Validators, merged, entries, m.ifaceChecker, m.validators); err != nil {
			return err
		}
	}

	// Apply props
	if err := applyRouteProps(entry, props); err != nil {
		return err
	}

	return nil
}

// Remove deletes a route entry.
func (m *RouteModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("route: entry %q not found", id)
	}
	if entry.dynamic {
		return fmt.Errorf("route: cannot remove dynamic entry %q", id)
	}

	delete(m.entries, id)
	return nil
}

// List returns all entries sorted by index.
func (m *RouteModule) List() []core.Entry {
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
func (m *RouteModule) Get(id string) (core.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[id]
	if !exists {
		return nil, false
	}
	return &entryAdapter{entry: entry}, true
}

// ---------------------------------------------------------------------------
// Public methods for /ip/address integration
// ---------------------------------------------------------------------------

// AddConnectedRoute creates a dynamic connected route for a directly attached
// subnet. This is called by /ip/address when a new IP address is added.
// The route is created with dynamic=true, active=true, distance=0.
func (m *RouteModule) AddConnectedRoute(network string, ifaceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &routeEntry{
		id:           strconv.Itoa(m.index),
		index:        m.index,
		dstAddress:   network,
		gateway:      ifaceName,
		distance:     0,
		prefSrc:      "",
		routingTable: "main",
		disabled:     false,
		blackhole:    false,
		checkGateway: "none",
		dynamic:      true,
		active:       true,
		connect:      true,
	}
	m.index++

	m.entries[entry.id] = entry
}

// RemoveConnectedRoute removes a dynamic connected route by network address.
// Called by /ip/address when an IP address is removed.
func (m *RouteModule) RemoveConnectedRoute(network string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, entry := range m.entries {
		if entry.dstAddress == network && entry.dynamic && entry.connect {
			delete(m.entries, id)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Lookup — find best matching route for a destination IP
// (longest prefix match among active routes)
// ---------------------------------------------------------------------------

// LookupResult contains the resolved gateway, outbound interface, and distance
// for a destination IP lookup.
type LookupResult struct {
	Gateway      string
	OutInterface string
	Distance     int
}

// Lookup finds the best matching route for the given destination IP address
// using longest prefix match among active routes.
// Returns nil if no matching route is found.
func (m *RouteModule) Lookup(dstIP string) *LookupResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dst := net.ParseIP(dstIP)
	if dst == nil {
		return nil
	}

	var bestMatch *routeEntry
	var bestPrefixLen int

	for _, entry := range m.entries {
		if entry.disabled || !entry.active {
			continue
		}

		_, cidr, err := net.ParseCIDR(entry.dstAddress)
		if err != nil {
			continue
		}

		if !cidr.Contains(dst) {
			continue
		}

		prefixLen, _ := cidr.Mask.Size()
		if bestMatch == nil || prefixLen > bestPrefixLen {
			bestMatch = entry
			bestPrefixLen = prefixLen
		}
	}

	if bestMatch == nil {
		return nil
	}

	gw := bestMatch.gateway
	outIface := ""

	// If gateway is an interface name, use it as outInterface
	if net.ParseIP(gw) == nil {
		outIface = gw
	} else {
		// Gateway is an IP, try to find matching connected route for outInterface
		for _, entry := range m.entries {
			if !entry.dynamic || !entry.connect {
				continue
			}
			_, cidr, err := net.ParseCIDR(entry.dstAddress)
			if err != nil {
				continue
			}
			gwIP := net.ParseIP(gw)
			if gwIP != nil && cidr.Contains(gwIP) {
				outIface = entry.gateway
				break
			}
		}
	}

	return &LookupResult{
		Gateway:      gw,
		OutInterface: outIface,
		Distance:     bestMatch.distance,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// applyRouteProps applies writable properties to a routeEntry.
func applyRouteProps(entry *routeEntry, props map[string]interface{}) error {
	for name, rawVal := range props {
		switch name {
		case "dst-address":
			s, ok := rawVal.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return fmt.Errorf("route: dst-address must be a non-empty string")
			}
			s = strings.TrimSpace(s)
			if err := validateCIDR(s); err != nil {
				return fmt.Errorf("route: %w", err)
			}
			entry.dstAddress = s
		case "gateway":
			s, ok := rawVal.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return fmt.Errorf("route: gateway must be a non-empty string")
			}
			s = strings.TrimSpace(s)
			entry.gateway = s
			// Re-activate if not disabled and not blackhole
			if !entry.disabled && !entry.blackhole {
				entry.active = true
			}
		case "distance":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("route: distance: %w", err)
			}
			if i < 0 || i > 255 {
				return fmt.Errorf("route: distance must be between 0 and 255")
			}
			entry.distance = i
		case "pref-src":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("route: pref-src must be a string")
			}
			entry.prefSrc = s
		case "routing-table":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("route: routing-table must be a string")
			}
			entry.routingTable = s
		case "disabled":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("route: disabled: %w", err)
			}
			entry.disabled = b
			if b {
				entry.active = false
			} else if !entry.blackhole {
				entry.active = true
			}
		case "blackhole":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("route: blackhole: %w", err)
			}
			entry.blackhole = b
			if b && !entry.disabled {
				entry.active = true
			}
		case "check-gateway":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("route: check-gateway must be a string")
			}
			entry.checkGateway = s
		}
	}
	return nil
}

// validateCIDR checks if the given string is a valid CIDR notation.
func validateCIDR(s string) error {
	_, _, err := net.ParseCIDR(s)
	if err != nil {
		return fmt.Errorf("invalid dst-address %q: must be valid CIDR notation", s)
	}
	return nil
}

// validateGateway checks if gateway is a valid IP address or existing interface.
func (m *RouteModule) validateGateway(gw string) error {
	// Check if it's a valid IP
	if net.ParseIP(gw) != nil {
		return nil
	}

	// Check if it's an existing interface name
	if m.ifaceChecker != nil && m.ifaceChecker.InterfaceExists(gw) {
		return nil
	}

	return fmt.Errorf("invalid gateway %q: must be a valid IP address or existing interface name", gw)
}

// entryList returns all entries as a sorted []core.Entry slice.
// Caller must hold at least a read lock.
func (m *RouteModule) entryList() []core.Entry {
	result := make([]core.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, &entryAdapter{entry: e})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Index() < result[j].Index()
	})
	return result
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
