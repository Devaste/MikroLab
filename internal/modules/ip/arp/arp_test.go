// Package ip_arp_test provides unit tests for the ARP module.
package ip_arp

import (
	"testing"
)

// mockInterfaceChecker implements InterfaceChecker for testing.
type mockInterfaceChecker struct {
	interfaces map[string]bool
}

func (m *mockInterfaceChecker) InterfaceExists(name string) bool {
	return m.interfaces[name]
}

func (m *mockInterfaceChecker) ListInterfaces() []string {
	names := make([]string, 0, len(m.interfaces))
	for name := range m.interfaces {
		names = append(names, name)
	}
	return names
}

func newMockChecker(ifaces ...string) *mockInterfaceChecker {
	m := &mockInterfaceChecker{interfaces: make(map[string]bool)}
	for _, iface := range ifaces {
		m.interfaces[iface] = true
	}
	return m
}

func newTestModule(t *testing.T) *ArpModule {
	t.Helper()
	mod, err := New("/ip/arp", "ARP Table", newMockChecker("ether1", "ether2", "bridge1"))
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}
	return mod
}

// TestArpAddValidEntry tests adding a valid ARP entry.
func TestArpAddValidEntry(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ID() == "" {
		t.Fatal("expected non-empty ID")
	}

	addr, _ := entry.Property("address")
	if addr != "192.168.1.1" {
		t.Errorf("expected address '192.168.1.1', got %v", addr)
	}

	mac, _ := entry.Property("mac-address")
	if mac != "64:D1:54:AA:BB:CC" {
		t.Errorf("expected MAC '64:D1:54:AA:BB:CC', got %v", mac)
	}

	iface, _ := entry.Property("interface")
	if iface != "ether1" {
		t.Errorf("expected interface 'ether1', got %v", iface)
	}

	published, _ := entry.Property("published")
	if published != false {
		t.Errorf("expected published=false, got %v", published)
	}

	disabled := entry.Disabled()
	if disabled {
		t.Errorf("expected disabled=false, got %v", disabled)
	}

	dynamic := entry.Dynamic()
	if dynamic {
		t.Errorf("expected dynamic=false, got %v", dynamic)
	}

	status, _ := entry.Property("status")
	if status != "complete" {
		t.Errorf("expected status='complete', got %v", status)
	}
}

// TestArpAddMissingRequiredProperties tests that missing required properties fail.
func TestArpAddMissingRequiredProperties(t *testing.T) {
	mod := newTestModule(t)

	// Missing address
	_, err := mod.Add(map[string]interface{}{
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err == nil {
		t.Error("expected error for missing address")
	}

	// Missing mac-address
	_, err = mod.Add(map[string]interface{}{
		"address":   "192.168.1.1",
		"interface": "ether1",
	})
	if err == nil {
		t.Error("expected error for missing mac-address")
	}

	// Missing interface
	_, err = mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
	})
	if err == nil {
		t.Error("expected error for missing interface")
	}
}

// TestArpAddInvalidIP tests that invalid IP addresses are rejected.
func TestArpAddInvalidIP(t *testing.T) {
	mod := newTestModule(t)

	invalidIPs := []string{
		"invalid",
		"256.256.256.256",
		"",
		"192.168.1",
	}

	for _, ip := range invalidIPs {
		_, err := mod.Add(map[string]interface{}{
			"address":     ip,
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		})
		if err == nil {
			t.Errorf("expected error for invalid IP %q", ip)
		}
	}
}

// TestArpAddInvalidMAC tests that invalid MAC addresses are rejected.
func TestArpAddInvalidMAC(t *testing.T) {
	mod := newTestModule(t)

	invalidMACs := []string{
		"invalid",
		"00:11:22:33:44:GG", // invalid hex
		"00:11:22:33:44",    // too short
		"",                  // empty
	}

	for _, mac := range invalidMACs {
		_, err := mod.Add(map[string]interface{}{
			"address":     "192.168.1.1",
			"mac-address": mac,
			"interface":   "ether1",
		})
		if err == nil {
			t.Errorf("expected error for invalid MAC %q", mac)
		}
	}
}

// TestArpAddValidMACFormats tests that all valid MAC formats are accepted.
func TestArpAddValidMACFormats(t *testing.T) {
	mod := newTestModule(t)

	validMACs := []struct {
		input    string
		expected string
	}{
		{"00:11:22:33:44:55", "00:11:22:33:44:55"},
		{"aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF"},
		{"00-11-22-33-44-55", "00:11:22:33:44:55"},
		{"0011.2233.4455", "00:11:22:33:44:55"},
	}

	for _, tc := range validMACs {
		entry, err := mod.Add(map[string]interface{}{
			"address":     "192.168.1.1",
			"mac-address": tc.input,
			"interface":   "ether1",
		})
		if err != nil {
			t.Errorf("unexpected error for valid MAC %q: %v", tc.input, err)
			continue
		}
		mac, _ := entry.Property("mac-address")
		if mac != tc.expected {
			t.Errorf("for input %q, expected normalized MAC %q, got %v", tc.input, tc.expected, mac)
		}

		// Remove the entry to avoid duplicate on same interface
		if err := mod.Remove(entry.ID()); err != nil {
			t.Fatalf("failed to remove entry: %v", err)
		}
	}
}

// TestArpAddInterfaceNotExists tests that non-existent interfaces are rejected.
func TestArpAddInterfaceNotExists(t *testing.T) {
	mod := newTestModule(t)

	_, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "nonexistent",
	})
	if err == nil {
		t.Error("expected error for non-existent interface")
	}
}

// TestArpAddDuplicateIPOnSameInterface tests that duplicate IPs are rejected.
func TestArpAddDuplicateIPOnSameInterface(t *testing.T) {
	mod := newTestModule(t)

	// First entry should succeed
	_, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error on first add: %v", err)
	}

	// Second entry with same IP but different MAC should fail
	_, err = mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "00:11:22:33:44:55",
		"interface":   "ether1",
	})
	if err == nil {
		t.Error("expected error for duplicate IP on same interface")
	}
}

// TestArpAddSameIPDifferentInterface tests that same IP on different interfaces is allowed.
func TestArpAddSameIPDifferentInterface(t *testing.T) {
	mod := newTestModule(t)

	// First entry on ether1
	_, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error on first add: %v", err)
	}

	// Same IP on ether2 should succeed
	_, err = mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "00:11:22:33:44:55",
		"interface":   "ether2",
	})
	if err != nil {
		t.Errorf("expected same IP on different interface to succeed, got: %v", err)
	}
}

// TestArpSet tests updating ARP entry properties.
func TestArpSet(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id := entry.ID()

	// Update MAC and published
	err = mod.Set(id, map[string]interface{}{
		"mac-address": "00:11:22:33:44:55",
		"published":   true,
	})
	if err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	// Verify changes
	updated, ok := mod.Get(id)
	if !ok {
		t.Fatal("entry not found after set")
	}

	mac, _ := updated.Property("mac-address")
	if mac != "00:11:22:33:44:55" {
		t.Errorf("expected MAC '00:11:22:33:44:55', got %v", mac)
	}

	published, _ := updated.Property("published")
	if published != true {
		t.Errorf("expected published=true, got %v", published)
	}

	// Address should remain unchanged
	addr, _ := updated.Property("address")
	if addr != "192.168.1.1" {
		t.Errorf("expected address to remain '192.168.1.1', got %v", addr)
	}
}

// TestArpRemove tests removing an ARP entry.
func TestArpRemove(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id := entry.ID()

	err = mod.Remove(id)
	if err != nil {
		t.Fatalf("unexpected error on remove: %v", err)
	}

	// Verify removal
	_, ok := mod.Get(id)
	if ok {
		t.Error("entry should not exist after removal")
	}

	// List should be empty
	entries := mod.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestArpRemoveNonExistent tests removing a non-existent entry.
func TestArpRemoveNonExistent(t *testing.T) {
	mod := newTestModule(t)

	err := mod.Remove("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

// TestArpListEntries tests listing ARP entries.
func TestArpListEntries(t *testing.T) {
	mod := newTestModule(t)

	// Add multiple entries
	_, _ = mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	_, _ = mod.Add(map[string]interface{}{
		"address":     "192.168.1.2",
		"mac-address": "64:D1:54:AA:BB:DD",
		"interface":   "ether1",
	})
	_, _ = mod.Add(map[string]interface{}{
		"address":     "10.0.0.1",
		"mac-address": "00:0C:42:11:22:33",
		"interface":   "ether2",
	})

	entries := mod.List()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify order by index
	for i, e := range entries {
		if e.Index() != i {
			t.Errorf("expected entry at position %d to have index %d, got %d", i, i, e.Index())
		}
	}

	// Verify first entry
	e0 := entries[0]
	addr, _ := e0.Property("address")
	if addr != "192.168.1.1" {
		t.Errorf("expected first entry address '192.168.1.1', got %v", addr)
	}
}

// TestArpGetEntry tests getting a single entry.
func TestArpGetEntry(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id := entry.ID()

	// Get by ID
	found, ok := mod.Get(id)
	if !ok {
		t.Fatal("expected to find entry")
	}

	addr, _ := found.Property("address")
	if addr != "192.168.1.1" {
		t.Errorf("expected address '192.168.1.1', got %v", addr)
	}

	// Get non-existent
	_, ok = mod.Get("nonexistent")
	if ok {
		t.Error("expected non-existent entry to return false")
	}
}

// TestArpResolve tests the Resolve method.
func TestArpResolve(t *testing.T) {
	mod := newTestModule(t)

	// Add entries
	_, _ = mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	_, _ = mod.Add(map[string]interface{}{
		"address":     "192.168.1.2",
		"mac-address": "00:11:22:33:44:55",
		"interface":   "ether1",
	})
	_, _ = mod.Add(map[string]interface{}{
		"address":     "10.0.0.1",
		"mac-address": "00:0C:42:11:22:33",
		"interface":   "ether2",
	})

	// Resolve exact match
	mac, ok := mod.Resolve("192.168.1.1", "ether1")
	if !ok {
		t.Fatal("expected to resolve 192.168.1.1 on ether1")
	}
	if mac != "64:D1:54:AA:BB:CC" {
		t.Errorf("expected MAC '64:D1:54:AA:BB:CC', got %s", mac)
	}

	// Resolve with wrong interface
	_, ok = mod.Resolve("192.168.1.1", "ether2")
	if ok {
		t.Error("expected no resolution for wrong interface")
	}

	// Resolve with empty interface (any)
	mac, ok = mod.Resolve("192.168.1.1", "")
	if !ok {
		t.Fatal("expected to resolve with empty interface")
	}
	if mac != "64:D1:54:AA:BB:CC" {
		t.Errorf("expected MAC '64:D1:54:AA:BB:CC', got %s", mac)
	}

	// Resolve non-existent IP
	_, ok = mod.Resolve("192.168.1.99", "ether1")
	if ok {
		t.Error("expected no resolution for non-existent IP")
	}
}

// TestArpFlags tests flag output.
func TestArpFlags(t *testing.T) {
	mod := newTestModule(t)

	// Normal static entry
	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	flags := entry.Flags()
	if flags["disabled"] {
		t.Error("expected disabled=false")
	}
	if flags["dynamic"] {
		t.Error("expected dynamic=false")
	}
	if !flags["complete"] {
		t.Error("expected complete=true")
	}
	if flags["dhcp"] {
		t.Error("expected dhcp=false")
	}
	if flags["published"] {
		t.Error("expected published=false")
	}
}

// TestArpFlagsPublished tests the published flag.
func TestArpFlagsPublished(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
		"published":   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	flags := entry.Flags()
	if !flags["published"] {
		t.Error("expected published=true")
	}
}

// TestGenerateMikroTikMAC tests the MAC generator produces valid MACs.
func TestGenerateMikroTikMAC(t *testing.T) {
	for i := 0; i < 100; i++ {
		mac := GenerateMikroTikMAC()
		if !isValidMAC(mac) {
			t.Errorf("generated invalid MAC: %s", mac)
		}
		// Check that it starts with one of the known OUIs
		if len(mac) >= 8 {
			prefix := mac[:8]
			if prefix != "64:D1:54" && prefix != "00:0C:42" {
				t.Errorf("generated MAC %s does not start with a known MikroTik OUI", mac)
			}
		}
	}
}

// TestArpSetCannotChangeInterface tests that interface cannot be changed via Set.
func TestArpSetCannotChangeInterface(t *testing.T) {
	mod := newTestModule(t)

	entry, err := mod.Add(map[string]interface{}{
		"address":     "192.168.1.1",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	id := entry.ID()

	// Try to change interface via Set - should be silently ignored in applyARPProps
	err = mod.Set(id, map[string]interface{}{
		"interface": "ether2",
	})
	if err != nil {
		t.Fatalf("set with interface change should not fail: %v", err)
	}

	// Verify interface is unchanged
	updated, _ := mod.Get(id)
	iface, _ := updated.Property("interface")
	if iface != "ether1" {
		t.Errorf("expected interface to remain 'ether1', got %v", iface)
	}
}
