// Package filter implements the /ip/firewall/filter settings directory
// for MikroLab's RouterOS v7 MVP firewall.
package filter

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/topology"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// Protocol numbers
const (
	ProtoICMP = 1
	ProtoTCP  = 6
	ProtoUDP  = 17
)

// Action constants
const (
	ActionAccept = "accept"
	ActionDrop   = "drop"
	ActionLog    = "log"
)

// Chain constants
const (
	ChainInput   = "input"
	ChainForward = "forward"
	ChainOutput  = "output"
)

// ---------------------------------------------------------------------------
// Rule — represents a single firewall filter rule
// ---------------------------------------------------------------------------

// Rule stores all properties for a single firewall filter rule.
type Rule struct {
	id           string
	index        int
	chain        string
	srcAddress   string // CIDR or single IP
	dstAddress   string // CIDR or single IP
	protocol     string // "tcp", "udp", "icmp", "all"
	srcPort      int    // for TCP/UDP
	dstPort      int    // for TCP/UDP
	inInterface  string
	outInterface string
	action       string // "accept", "drop", "log"
	disabled     bool
	comment      string
	logPrefix    string
}

// ---------------------------------------------------------------------------
// entryAdapter — wraps Rule to satisfy core.Entry
// ---------------------------------------------------------------------------

type entryAdapter struct {
	rule *Rule
}

func (a *entryAdapter) ID() string     { return a.rule.id }
func (a *entryAdapter) Index() int     { return a.rule.index }
func (a *entryAdapter) Disabled() bool { return a.rule.disabled }
func (a *entryAdapter) Dynamic() bool  { return false }
func (a *entryAdapter) Invalid() bool  { return false }
func (a *entryAdapter) Slave() bool    { return false }

func (a *entryAdapter) Property(name string) (interface{}, bool) {
	switch name {
	case "chain":
		return a.rule.chain, true
	case "src-address":
		return a.rule.srcAddress, true
	case "dst-address":
		return a.rule.dstAddress, true
	case "protocol":
		return a.rule.protocol, true
	case "src-port":
		return a.rule.srcPort, true
	case "dst-port":
		return a.rule.dstPort, true
	case "in-interface":
		return a.rule.inInterface, true
	case "out-interface":
		return a.rule.outInterface, true
	case "action":
		return a.rule.action, true
	case "disabled":
		return a.rule.disabled, true
	case "comment":
		return a.rule.comment, true
	case "log-prefix":
		return a.rule.logPrefix, true
	default:
		return nil, false
	}
}

func (a *entryAdapter) SetProperty(name string, value interface{}) error {
	switch name {
	case "chain":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: chain must be a string")
		}
		if !isValidChain(s) {
			return fmt.Errorf("filter: invalid chain %q", s)
		}
		a.rule.chain = s
	case "src-address":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: src-address must be a string")
		}
		a.rule.srcAddress = s
	case "dst-address":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: dst-address must be a string")
		}
		a.rule.dstAddress = s
	case "protocol":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: protocol must be a string")
		}
		if !isValidProtocol(s) {
			return fmt.Errorf("filter: invalid protocol %q", s)
		}
		a.rule.protocol = s
	case "src-port":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("filter: src-port: %w", err)
		}
		if i < 1 || i > 65535 {
			return fmt.Errorf("filter: src-port must be between 1 and 65535")
		}
		a.rule.srcPort = i
	case "dst-port":
		i, err := toInt(value)
		if err != nil {
			return fmt.Errorf("filter: dst-port: %w", err)
		}
		if i < 1 || i > 65535 {
			return fmt.Errorf("filter: dst-port must be between 1 and 65535")
		}
		a.rule.dstPort = i
	case "in-interface":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: in-interface must be a string")
		}
		a.rule.inInterface = s
	case "out-interface":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: out-interface must be a string")
		}
		a.rule.outInterface = s
	case "action":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: action must be a string")
		}
		if !isValidAction(s) {
			return fmt.Errorf("filter: invalid action %q", s)
		}
		a.rule.action = s
	case "disabled":
		b, err := toBool(value)
		if err != nil {
			return fmt.Errorf("filter: disabled: %w", err)
		}
		a.rule.disabled = b
	case "comment":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: comment must be a string")
		}
		a.rule.comment = s
	case "log-prefix":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("filter: log-prefix must be a string")
		}
		a.rule.logPrefix = s
	default:
		return fmt.Errorf("filter: property %q does not exist or is read-only", name)
	}
	return nil
}

func (a *entryAdapter) Properties() map[string]interface{} {
	return map[string]interface{}{
		"chain":         a.rule.chain,
		"src-address":   a.rule.srcAddress,
		"dst-address":   a.rule.dstAddress,
		"protocol":      a.rule.protocol,
		"src-port":      a.rule.srcPort,
		"dst-port":      a.rule.dstPort,
		"in-interface":  a.rule.inInterface,
		"out-interface": a.rule.outInterface,
		"action":        a.rule.action,
		"disabled":      a.rule.disabled,
		"comment":       a.rule.comment,
		"log-prefix":    a.rule.logPrefix,
	}
}

func (a *entryAdapter) Flags() map[string]bool {
	return map[string]bool{
		"disabled": a.rule.disabled,
		"invalid":  false,
	}
}

// Compile-time check
var _ core.Entry = (*entryAdapter)(nil)

// ---------------------------------------------------------------------------
// InterfaceChecker — interface for checking if an interface exists
// ---------------------------------------------------------------------------

// InterfaceChecker defines the interface for checking interface existence.
type InterfaceChecker interface {
	InterfaceExists(name string) bool
}

// LogAdder defines the interface for adding log entries.
type LogAdder interface {
	Add(entry string)
}

// ---------------------------------------------------------------------------
// FilterModule — implements core.SettingsDirectory for /ip/firewall/filter
// ---------------------------------------------------------------------------

// FilterModule provides full CRUD for firewall filter rules and packet evaluation.
type FilterModule struct {
	mu           sync.RWMutex
	path         string
	title        string
	schema       *config.ModuleSchema
	entries      map[string]*Rule // rule ID -> rule
	byID         map[string]int   // rule ID -> index in rules slice
	rules        []*Rule          // ordered list
	index        int
	ifaceChecker InterfaceChecker
	logAdder     LogAdder
}

// New creates a new FilterModule with the given schema.
// ifaceChecker is optional; if non-nil, interface name validation is performed.
// logAdder is optional; if non-nil, log entries will be recorded.
func New(schema *config.ModuleSchema, ifaceChecker InterfaceChecker, logAdder LogAdder) (*FilterModule, error) {
	if schema == nil {
		return nil, fmt.Errorf("filter: schema is required")
	}
	return &FilterModule{
		path:         schema.Path,
		title:        schema.Title,
		schema:       schema,
		entries:      make(map[string]*Rule),
		byID:         make(map[string]int),
		rules:        make([]*Rule, 0),
		ifaceChecker: ifaceChecker,
		logAdder:     logAdder,
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

// Path returns the full absolute path "/ip/firewall/filter".
func (m *FilterModule) Path() string { return m.path }

// Type returns core.NodeTypeList.
func (m *FilterModule) Type() core.NodeType { return core.NodeTypeList }

// Title returns the human-readable display name.
func (m *FilterModule) Title() string { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

// Children returns nil — /ip/firewall/filter is a leaf list node.
func (m *FilterModule) Children() map[string]core.Node { return nil }

// AddChild returns an error — a list node cannot contain children.
func (m *FilterModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("filter: list node %q cannot accept child %q", m.path, name)
}

// RemoveChild returns an error — a list node has no children.
func (m *FilterModule) RemoveChild(name string) error {
	return fmt.Errorf("filter: list node %q has no child %q", m.path, name)
}

// Child returns (nil, false) — /ip/firewall/filter has no child nodes.
func (m *FilterModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.SettingsDirectory interface
// ---------------------------------------------------------------------------

// Add creates a new filter rule.
func (m *FilterModule) Add(props map[string]interface{}) (core.Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Validate and extract properties
	chainRaw, hasChain := props["chain"]
	if !hasChain {
		return nil, fmt.Errorf("filter: required property %q is missing", "chain")
	}
	chain, ok := chainRaw.(string)
	if !ok || !isValidChain(chain) {
		return nil, fmt.Errorf("filter: invalid chain %q", chain)
	}
	actionRaw, hasAction := props["action"]
	if !hasAction {
		return nil, fmt.Errorf("filter: required property %q is missing", "action")
	}
	action, ok := actionRaw.(string)
	if !ok || !isValidAction(action) {
		return nil, fmt.Errorf("filter: invalid action %q", action)
	}

	// 2. Build the rule
	rule := &Rule{
		id:       uuid.New().String(),
		index:    m.index,
		chain:    chain,
		action:   action,
		disabled: false,
	}
	m.index++

	// 3. Apply defaults
	rule.protocol = "all"

	// 4. Apply provided properties
	if err := applyRuleProps(rule, props, m.ifaceChecker); err != nil {
		return nil, err
	}

	// 5. Store the rule
	m.entries[rule.id] = rule
	m.byID[rule.id] = len(m.rules)
	m.rules = append(m.rules, rule)

	return &entryAdapter{rule: rule}, nil
}

// Set updates properties of an existing rule identified by its ID.
func (m *FilterModule) Set(id string, props map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("filter: entry %q not found", id)
	}

	if err := applyRuleProps(rule, props, m.ifaceChecker); err != nil {
		return err
	}

	return nil
}

// Remove deletes the rule with the given ID.
func (m *FilterModule) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.entries[id]
	if !exists {
		return fmt.Errorf("filter: entry %q not found", id)
	}

	// Remove from rules slice
	pos := m.byID[id]
	m.rules = append(m.rules[:pos], m.rules[pos+1:]...)

	// Update byID map for shifted rules
	for i := pos; i < len(m.rules); i++ {
		m.byID[m.rules[i].id] = i
	}

	delete(m.entries, id)
	delete(m.byID, id)
	return nil
}

// List returns all rules ordered by their index.
func (m *FilterModule) List() []core.Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.entryList()
}

// Get returns the rule with the given ID, or false if not found.
func (m *FilterModule) Get(id string) (core.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.entries[id]
	if !exists {
		return nil, false
	}
	return &entryAdapter{rule: rule}, true
}

// ---------------------------------------------------------------------------
// Firewall evaluation
// ---------------------------------------------------------------------------

// Evaluate checks a packet against the rule list for the given chain.
// Returns the action ("accept", "drop", "log") and a log message (if action is "log").
// If no rule matches, returns ("accept", "") as the default policy.
func (m *FilterModule) Evaluate(deviceID, chain string, packet topology.Packet) (action string, logMsg string) {
	m.mu.RLock()
	rules := m.rules
	m.mu.RUnlock()

	// Use interfaces from packet metadata
	inIface := packet.InIface
	outIface := packet.OutIface

	for _, rule := range rules {
		if rule.disabled {
			continue
		}
		if rule.chain != chain {
			continue
		}

		if !matchRule(rule, packet, inIface, outIface) {
			continue
		}

		// Rule matched
		switch rule.action {
		case ActionAccept:
			return ActionAccept, ""
		case ActionDrop:
			return ActionDrop, ""
		case ActionLog:
			msg := formatLogMessage(rule, packet, deviceID)
			m.AddLogEntry(msg)
			return ActionLog, msg
		}
	}

	// Default: accept
	return ActionAccept, ""
}

// SetLogAdder sets the log adder for recording log action messages.
func (m *FilterModule) SetLogAdder(adder LogAdder) error {
	if adder == nil {
		return fmt.Errorf("filter: log adder cannot be nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logAdder = adder
	return nil
}

// GetRules returns a copy of the rules slice (for evaluation by external callers).
func (m *FilterModule) GetRules() []*Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*Rule, len(m.rules))
	for i, r := range m.rules {
		ruleCopy := *r
		rules[i] = &ruleCopy
	}
	return rules
}

// AddLogEntry adds a log entry to the attached log module.
func (m *FilterModule) AddLogEntry(entry string) {
	m.mu.RLock()
	adder := m.logAdder
	m.mu.RUnlock()
	if adder != nil {
		adder.Add(entry)
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// entryList returns all entries as a sorted []core.Entry slice.
// Caller must hold at least a read lock.
func (m *FilterModule) entryList() []core.Entry {
	result := make([]core.Entry, 0, len(m.rules))
	for _, rule := range m.rules {
		result = append(result, &entryAdapter{rule: rule})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Index() < result[j].Index()
	})
	return result
}

// applyRuleProps applies writable properties to a Rule.
func applyRuleProps(rule *Rule, props map[string]interface{}, ifaceChecker InterfaceChecker) error {
	for name, rawVal := range props {
		switch name {
		case "chain":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: chain must be a string")
			}
			if !isValidChain(s) {
				return fmt.Errorf("filter: invalid chain %q", s)
			}
			rule.chain = s
		case "src-address":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: src-address must be a string")
			}
			rule.srcAddress = s
		case "dst-address":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: dst-address must be a string")
			}
			rule.dstAddress = s
		case "protocol":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: protocol must be a string")
			}
			if !isValidProtocol(s) {
				return fmt.Errorf("filter: invalid protocol %q", s)
			}
			rule.protocol = s
		case "src-port":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("filter: src-port: %w", err)
			}
			if i < 1 || i > 65535 {
				return fmt.Errorf("filter: src-port must be between 1 and 65535")
			}
			rule.srcPort = i
		case "dst-port":
			i, err := toInt(rawVal)
			if err != nil {
				return fmt.Errorf("filter: dst-port: %w", err)
			}
			if i < 1 || i > 65535 {
				return fmt.Errorf("filter: dst-port must be between 1 and 65535")
			}
			rule.dstPort = i
		case "in-interface":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: in-interface must be a string")
			}
			if ifaceChecker != nil && s != "" && !ifaceChecker.InterfaceExists(s) {
				return fmt.Errorf("filter: interface %q does not exist", s)
			}
			rule.inInterface = s
		case "out-interface":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: out-interface must be a string")
			}
			if ifaceChecker != nil && s != "" && !ifaceChecker.InterfaceExists(s) {
				return fmt.Errorf("filter: interface %q does not exist", s)
			}
			rule.outInterface = s
		case "action":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: action must be a string")
			}
			if !isValidAction(s) {
				return fmt.Errorf("filter: invalid action %q", s)
			}
			rule.action = s
		case "disabled":
			b, err := toBool(rawVal)
			if err != nil {
				return fmt.Errorf("filter: disabled: %w", err)
			}
			rule.disabled = b
		case "comment":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: comment must be a string")
			}
			rule.comment = s
		case "log-prefix":
			s, ok := rawVal.(string)
			if !ok {
				return fmt.Errorf("filter: log-prefix must be a string")
			}
			rule.logPrefix = s
		case "numbers", "id":
			// handled by CLI layer
		default:
			// Silently ignore unknown properties (RouterOS behavior)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Matching
// ---------------------------------------------------------------------------

// matchRule checks if a packet matches a rule's criteria.
func matchRule(rule *Rule, packet topology.Packet, inIface, outIface string) bool {
	// 1. Match in-interface
	if rule.inInterface != "" {
		if rule.inInterface != inIface {
			return false
		}
	}

	// 2. Match out-interface
	if rule.outInterface != "" {
		if rule.outInterface != outIface {
			return false
		}
	}

	// 3. Match source address
	if rule.srcAddress != "" && rule.srcAddress != "0.0.0.0/0" {
		if !ipMatchesCIDR(packet.SrcIP, rule.srcAddress) {
			return false
		}
	}

	// 4. Match destination address
	if rule.dstAddress != "" && rule.dstAddress != "0.0.0.0/0" {
		if !ipMatchesCIDR(packet.DstIP, rule.dstAddress) {
			return false
		}
	}

	// 5. Match protocol
	if rule.protocol != "all" {
		protoNum := protocolToNumber(rule.protocol)
		if packet.Protocol != protoNum {
			return false
		}
	}

	// 6. Match ports (only for TCP/UDP)
	if rule.srcPort > 0 || rule.dstPort > 0 {
		// Ports are only meaningful for TCP and UDP
		if packet.Protocol != ProtoTCP && packet.Protocol != ProtoUDP {
			return false
		}
		if rule.srcPort > 0 {
			if packet.SrcPort != rule.srcPort {
				return false
			}
		}
		if rule.dstPort > 0 {
			if packet.DstPort != rule.dstPort {
				return false
			}
		}
	}

	return true
}

// ipMatchesCIDR checks if ip is within the given CIDR range.
// If cidr is a single IP (no prefix), it checks for exact match.
func ipMatchesCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// If no /, treat as exact IP match
	if !strings.Contains(cidr, "/") {
		return ip.Equal(net.ParseIP(cidr))
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return ipNet.Contains(ip)
}

// protocolToNumber converts a protocol string to its IP protocol number.
func protocolToNumber(proto string) int {
	switch strings.ToLower(proto) {
	case "icmp":
		return ProtoICMP
	case "tcp":
		return ProtoTCP
	case "udp":
		return ProtoUDP
	}
	return 0
}

// protocolToString converts an IP protocol number to a string.
func protocolToString(proto int) string {
	switch proto {
	case ProtoICMP:
		return "icmp"
	case ProtoTCP:
		return "tcp"
	case ProtoUDP:
		return "udp"
	default:
		return fmt.Sprintf("%d", proto)
	}
}

// formatLogMessage creates a log message for a matched log rule.
func formatLogMessage(rule *Rule, packet topology.Packet, deviceID string) string {
	prefix := rule.logPrefix
	if prefix != "" {
		prefix = prefix + " "
	}

	return fmt.Sprintf("%s%s: %s -> %s proto=%s in=%s out=%s",
		prefix,
		rule.chain,
		packet.SrcIP,
		packet.DstIP,
		protocolToString(packet.Protocol),
		packet.InIface,
		packet.OutIface,
	)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func isValidChain(chain string) bool {
	switch chain {
	case ChainInput, ChainForward, ChainOutput:
		return true
	}
	return false
}

func isValidAction(action string) bool {
	switch action {
	case ActionAccept, ActionDrop, ActionLog:
		return true
	}
	return false
}

func isValidProtocol(proto string) bool {
	switch strings.ToLower(proto) {
	case "tcp", "udp", "icmp", "all":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Type conversion helpers (mirroring interface module)
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
		var i int
		if _, err := fmt.Sscanf(val, "%d", &i); err != nil {
			return 0, fmt.Errorf("expected number, got %q", val)
		}
		return i, nil
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
