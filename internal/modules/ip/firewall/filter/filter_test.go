// Package filter_test contains unit tests for the firewall filter module.
package filter_test

import (
	"testing"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/modules/ip/firewall/filter"
	"github.com/Devaste/MikroLab/internal/topology"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mockIfaceChecker implements filter.InterfaceChecker for testing.
type mockIfaceChecker struct {
	interfaces map[string]bool
}

func (m *mockIfaceChecker) InterfaceExists(name string) bool {
	return m.interfaces[name]
}

// mockLogAdder implements filter.LogAdder for testing.
type mockLogAdder struct {
	entries []string
}

func (m *mockLogAdder) Add(entry string) {
	m.entries = append(m.entries, entry)
}

// newTestModule creates a FilterModule for testing.
func newTestModule(t *testing.T) *filter.FilterModule {
	t.Helper()

	schema := &config.ModuleSchema{
		Path:  "/ip/firewall/filter",
		Type:  "list",
		Title: "Firewall Filter Rules",
		Schema: map[string]*config.SchemaProperty{
			"chain":         {Name: "chain", Type: config.SchemaEnum, Required: true},
			"src-address":   {Name: "src-address", Type: config.SchemaString},
			"dst-address":   {Name: "dst-address", Type: config.SchemaString},
			"protocol":      {Name: "protocol", Type: config.SchemaEnum},
			"src-port":      {Name: "src-port", Type: config.SchemaInteger},
			"dst-port":      {Name: "dst-port", Type: config.SchemaInteger},
			"in-interface":  {Name: "in-interface", Type: config.SchemaString},
			"out-interface": {Name: "out-interface", Type: config.SchemaString},
			"action":        {Name: "action", Type: config.SchemaEnum, Required: true},
			"disabled":      {Name: "disabled", Type: config.SchemaBoolean},
			"comment":       {Name: "comment", Type: config.SchemaString},
			"log-prefix":    {Name: "log-prefix", Type: config.SchemaString},
		},
		Defaults: map[string]interface{}{
			"action":   "accept",
			"disabled": false,
		},
	}

	mod, err := filter.New(schema, nil, nil)
	if err != nil {
		t.Fatalf("failed to create FilterModule: %v", err)
	}
	return mod
}

// addRule is a helper to add a rule.
func addRule(t *testing.T, mod *filter.FilterModule, props map[string]interface{}) {
	t.Helper()
	_, err := mod.Add(props)
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}
}

// TestAddRuleOrdering verifies that rules maintain their insertion order.
func TestAddRuleOrdering(t *testing.T) {
	mod := newTestModule(t)

	addRule(t, mod, map[string]interface{}{
		"chain":  "input",
		"action": "accept",
	})
	addRule(t, mod, map[string]interface{}{
		"chain":  "input",
		"action": "drop",
	})
	addRule(t, mod, map[string]interface{}{
		"chain":  "forward",
		"action": "accept",
	})

	entries := mod.List()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify ordering
	expectedActions := []string{"accept", "drop", "accept"}
	for i, e := range entries {
		action, _ := e.Property("action")
		if action != expectedActions[i] {
			t.Errorf("rule %d: expected action %q, got %q", i, expectedActions[i], action)
		}
	}
}

// TestCIDRMatching verifies that IP CIDR matching works correctly.
func TestCIDRMatching(t *testing.T) {
	mod := newTestModule(t)

	// Add a rule that drops all traffic from 192.168.1.0/24
	addRule(t, mod, map[string]interface{}{
		"chain":       "input",
		"src-address": "192.168.1.0/24",
		"action":      "drop",
	})

	// Should match 192.168.1.5
	action, _ := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP: "192.168.1.5",
		DstIP: "10.0.0.1",
	})
	if action != "drop" {
		t.Errorf("expected drop for 192.168.1.5, got %q", action)
	}

	// Should NOT match 10.0.0.1
	action, _ = mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP: "10.0.0.1",
		DstIP: "192.168.1.5",
	})
	if action != "accept" {
		t.Errorf("expected accept for 10.0.0.1, got %q", action)
	}

	// Exact IP match (no CIDR)
	mod2 := newTestModule(t)
	addRule(t, mod2, map[string]interface{}{
		"chain":       "input",
		"src-address": "10.0.0.5",
		"action":      "drop",
	})

	action, _ = mod2.Evaluate("device-1", "input", topology.Packet{
		SrcIP: "10.0.0.5",
		DstIP: "192.168.1.1",
	})
	if action != "drop" {
		t.Errorf("expected drop for exact IP match, got %q", action)
	}

	action, _ = mod2.Evaluate("device-1", "input", topology.Packet{
		SrcIP: "10.0.0.6",
		DstIP: "192.168.1.1",
	})
	if action != "accept" {
		t.Errorf("expected accept for non-matching IP, got %q", action)
	}
}

// TestProtocolPortMatching verifies protocol and port matching.
func TestProtocolPortMatching(t *testing.T) {
	mod := newTestModule(t)

	addRule(t, mod, map[string]interface{}{
		"chain":    "input",
		"protocol": "tcp",
		"dst-port": 80,
		"action":   "drop",
	})

	// TCP/80 should match
	action, _ := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:    "10.0.0.1",
		DstIP:    "10.0.0.2",
		Protocol: 6, // TCP
		DstPort:  80,
	})
	if action != "drop" {
		t.Errorf("expected drop for TCP/80, got %q", action)
	}

	// UDP/80 should NOT match (different protocol)
	action, _ = mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:    "10.0.0.1",
		DstIP:    "10.0.0.2",
		Protocol: 17, // UDP
		DstPort:  80,
	})
	if action != "accept" {
		t.Errorf("expected accept for UDP/80, got %q", action)
	}

	// TCP/8080 should NOT match (different port)
	action, _ = mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:    "10.0.0.1",
		DstIP:    "10.0.0.2",
		Protocol: 6,
		DstPort:  8080,
	})
	if action != "accept" {
		t.Errorf("expected accept for TCP/8080, got %q", action)
	}
}

// TestInterfaceMatching verifies interface matching.
func TestInterfaceMatching(t *testing.T) {
	mod := newTestModule(t)

	addRule(t, mod, map[string]interface{}{
		"chain":        "input",
		"in-interface": "ether1",
		"action":       "drop",
	})

	// Packet arriving on ether1 should match
	action, _ := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:   "10.0.0.1",
		DstIP:   "10.0.0.2",
		InIface: "ether1",
	})
	if action != "drop" {
		t.Errorf("expected drop for in-interface ether1, got %q", action)
	}

	// Packet arriving on ether2 should NOT match
	action, _ = mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:   "10.0.0.1",
		DstIP:   "10.0.0.2",
		InIface: "ether2",
	})
	if action != "accept" {
		t.Errorf("expected accept for in-interface ether2, got %q", action)
	}
}

// TestDisabledRule verifies that disabled rules are skipped.
func TestDisabledRule(t *testing.T) {
	mod := newTestModule(t)

	// Add a disabled rule
	addRule(t, mod, map[string]interface{}{
		"chain":    "input",
		"protocol": "icmp",
		"action":   "drop",
		"disabled": true,
	})

	// ICMP should still be accepted (rule is disabled)
	action, _ := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:    "10.0.0.1",
		DstIP:    "10.0.0.2",
		Protocol: 1, // ICMP
	})
	if action != "accept" {
		t.Errorf("expected accept (disabled rule), got %q", action)
	}
}

// TestLogAction verifies that log action returns a log message.
func TestLogAction(t *testing.T) {
	mod := newTestModule(t)

	logAdder := &mockLogAdder{}
	mod.SetLogAdder(logAdder)

	addRule(t, mod, map[string]interface{}{
		"chain":       "input",
		"src-address": "192.168.1.0/24",
		"action":      "log",
		"log-prefix":  "TEST",
	})

	action, logMsg := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:   "192.168.1.5",
		DstIP:   "10.0.0.1",
		InIface: "ether1",
	})
	if action != "log" {
		t.Errorf("expected log action, got %q", action)
	}
	if logMsg == "" {
		t.Errorf("expected non-empty log message")
	}
	if len(logAdder.entries) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logAdder.entries))
	}
}

// TestDefaultAccept verifies that packets not matching any rule are accepted.
func TestDefaultAccept(t *testing.T) {
	mod := newTestModule(t)

	// Only drop ICMP
	addRule(t, mod, map[string]interface{}{
		"chain":    "input",
		"protocol": "icmp",
		"action":   "drop",
	})

	// TCP should be accepted (no matching rule)
	action, _ := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP:    "10.0.0.1",
		DstIP:    "10.0.0.2",
		Protocol: 6, // TCP
	})
	if action != "accept" {
		t.Errorf("expected accept for non-matching protocol (default), got %q", action)
	}
}

// TestChainSeparation verifies that rules from different chains don't interfere.
func TestChainSeparation(t *testing.T) {
	mod := newTestModule(t)

	// Add an input chain rule
	addRule(t, mod, map[string]interface{}{
		"chain":  "input",
		"action": "drop",
	})

	// Forward chain should still accept (no rules)
	action, _ := mod.Evaluate("device-1", "forward", topology.Packet{
		SrcIP: "10.0.0.1",
		DstIP: "10.0.0.2",
	})
	if action != "accept" {
		t.Errorf("expected accept for forward chain (no rules), got %q", action)
	}

	// Input chain should match
	action, _ = mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP: "10.0.0.1",
		DstIP: "10.0.0.2",
	})
	if action != "drop" {
		t.Errorf("expected drop for input chain, got %q", action)
	}
}

// TestRemoveRule verifies that removing a rule works.
func TestRemoveRule(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"chain":  "input",
		"action": "drop",
	})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	err = mod.Remove(entry.ID())
	if err != nil {
		t.Fatalf("failed to remove rule: %v", err)
	}

	if len(mod.List()) != 0 {
		t.Errorf("expected 0 rules after removal, got %d", len(mod.List()))
	}
}

// TestSetRule verifies that modifying a rule works.
func TestSetRule(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"chain":  "input",
		"action": "accept",
	})
	if err != nil {
		t.Fatalf("failed to add rule: %v", err)
	}

	err = mod.Set(entry.ID(), map[string]interface{}{
		"action": "drop",
	})
	if err != nil {
		t.Fatalf("failed to set rule: %v", err)
	}

	// Verify the change
	action, _ := mod.Evaluate("device-1", "input", topology.Packet{
		SrcIP: "10.0.0.1",
		DstIP: "10.0.0.2",
	})
	if action != "drop" {
		t.Errorf("expected drop after set, got %q", action)
	}
}

// TestOutputChain verifies output chain evaluation.
func TestOutputChain(t *testing.T) {
	mod := newTestModule(t)

	addRule(t, mod, map[string]interface{}{
		"chain":  "output",
		"action": "drop",
	})

	action, _ := mod.Evaluate("device-1", "output", topology.Packet{
		SrcIP: "10.0.0.1",
		DstIP: "10.0.0.2",
	})
	if action != "drop" {
		t.Errorf("expected drop for output chain, got %q", action)
	}
}
