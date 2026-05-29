package bridge

import (
	"testing"
	"time"
)

func TestAddBridge(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	entry, err := m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	name, _ := entry.Property("name")
	if name != "bridge1" {
		t.Errorf("expected name 'bridge1', got %v", name)
	}

	mtu, _ := entry.Property("mtu")
	if mtu != 1500 {
		t.Errorf("expected default MTU 1500, got %v", mtu)
	}
}

func TestAddBridgeDuplicate(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add first bridge: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err == nil {
		t.Fatal("expected error for duplicate bridge name")
	}
}

func TestRemoveBridge(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	err = m.RemoveBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to remove bridge: %v", err)
	}

	// Verify removal
	if m.GetBridge("bridge1") != nil {
		t.Fatal("expected bridge to be nil after removal")
	}
}

func TestRemoveNonExistentBridge(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	err = m.RemoveBridge("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent bridge")
	}
}

func TestBridgeExists(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	if m.BridgeExists("bridge1") {
		t.Fatal("expected BridgeExists to return false for non-existent bridge")
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	if !m.BridgeExists("bridge1") {
		t.Fatal("expected BridgeExists to return true for existing bridge")
	}
}

func TestAddPort(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	err = m.AddPort("bridge1", "ether1")
	if err != nil {
		t.Fatalf("failed to add port: %v", err)
	}

	ports := m.GetBridgePorts("bridge1")
	if len(ports) != 1 || ports[0] != "ether1" {
		t.Errorf("expected ports [ether1], got %v", ports)
	}

	// Check HasPort
	bridgeName, found := m.HasPort("ether1")
	if !found || bridgeName != "bridge1" {
		t.Errorf("expected HasPort to return (bridge1, true), got (%s, %v)", bridgeName, found)
	}
}

func TestRemovePort(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	err = m.AddPort("bridge1", "ether1")
	if err != nil {
		t.Fatalf("failed to add port: %v", err)
	}

	err = m.RemovePort("bridge1", "ether1")
	if err != nil {
		t.Fatalf("failed to remove port: %v", err)
	}

	ports := m.GetBridgePorts("bridge1")
	if len(ports) != 0 {
		t.Errorf("expected empty ports, got %v", ports)
	}

	_, found := m.HasPort("ether1")
	if found {
		t.Fatal("expected HasPort to return false after removal")
	}
}

func TestMACLearning(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	m.AddPort("bridge1", "ether1")
	m.AddPort("bridge1", "ether2")

	// Learn a MAC
	m.AddMAC("bridge1", "00:11:22:33:44:55", "ether1")

	// Lookup
	port, found := m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if !found || port != "ether1" {
		t.Errorf("expected LookupMAC to return (ether1, true), got (%s, %v)", port, found)
	}

	// Check forwarding table
	table := m.GetForwardingTable("bridge1")
	if len(table) != 1 {
		t.Errorf("expected 1 MAC in forwarding table, got %d", len(table))
	}

	// Remove MAC
	m.RemoveMAC("bridge1", "00:11:22:33:44:55")
	_, found = m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if found {
		t.Fatal("expected MAC to be removed")
	}
}

func TestMACAgeing(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	m.AddPort("bridge1", "ether1")

	// Override ageing time to be very short
	bridge := m.GetBridge("bridge1")
	bridge.ageingTime = 1 * time.Millisecond

	m.AddMAC("bridge1", "00:11:22:33:44:55", "ether1")

	// Verify it was added
	_, found := m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if !found {
		t.Fatal("expected MAC to be present before ageing")
	}

	// Wait for ageing
	time.Sleep(5 * time.Millisecond)

	// Age the MACs
	aged := m.AgeMACs("bridge1")
	if aged != 1 {
		t.Errorf("expected 1 aged entry, got %d", aged)
	}

	// Verify it's gone
	_, found = m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if found {
		t.Fatal("expected MAC to be aged out")
	}
}

func TestMACLearningUpdatesTimestamp(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	m.AddPort("bridge1", "ether1")

	bridge := m.GetBridge("bridge1")
	bridge.ageingTime = 5 * time.Minute

	// Learn a MAC
	m.AddMAC("bridge1", "00:11:22:33:44:55", "ether1")

	// Re-learn to update timestamp (would prevent ageing)
	m.AddMAC("bridge1", "00:11:22:33:44:55", "ether1")

	// Verify it's still present
	port, found := m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if !found {
		t.Fatal("expected MAC to be present")
	}
	if port != "ether1" {
		t.Errorf("expected port ether1, got %s", port)
	}
}

func TestMACLookupNonExistent(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	// Lookup unknown MAC
	_, found := m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if found {
		t.Fatal("expected LookupMAC to return false for unknown MAC")
	}

	// Lookup on unknown bridge
	_, found = m.LookupMAC("nonexistent", "00:11:22:33:44:55")
	if found {
		t.Fatal("expected LookupMAC to return false for unknown bridge")
	}
}

func TestPortMembership(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}
	_, err = m.AddBridge("bridge2")
	if err != nil {
		t.Fatalf("failed to add bridge2: %v", err)
	}

	m.AddPort("bridge1", "ether1")
	m.AddPort("bridge2", "ether2")

	// Check HasPort
	bridgeName, found := m.HasPort("ether1")
	if !found || bridgeName != "bridge1" {
		t.Errorf("expected HasPort(ether1) to return (bridge1, true), got (%s, %v)", bridgeName, found)
	}

	bridgeName, found = m.HasPort("ether2")
	if !found || bridgeName != "bridge2" {
		t.Errorf("expected HasPort(ether2) to return (bridge2, true), got (%s, %v)", bridgeName, found)
	}

	// Check port that is not in any bridge
	_, found = m.HasPort("ether3")
	if found {
		t.Fatal("expected HasPort(ether3) to return false")
	}
}

func TestRemovePortClearsMACs(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	m.AddPort("bridge1", "ether1")
	m.AddPort("bridge1", "ether2")

	// Learn MACs on both ports
	m.AddMAC("bridge1", "00:11:22:33:44:55", "ether1")
	m.AddMAC("bridge1", "AA:BB:CC:DD:EE:FF", "ether2")

	// Remove ether1 - its MACs should be cleared
	m.RemovePort("bridge1", "ether1")

	// Verify ether1's MAC is gone
	_, found := m.LookupMAC("bridge1", "00:11:22:33:44:55")
	if found {
		t.Fatal("expected MAC for removed port to be cleared")
	}

	// Verify ether2's MAC is still there
	_, found = m.LookupMAC("bridge1", "AA:BB:CC:DD:EE:FF")
	if !found {
		t.Fatal("expected MAC for remaining port to still be present")
	}
}

func TestGetBridgePorts(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	_, err = m.AddBridge("bridge1")
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	// No ports yet
	ports := m.GetBridgePorts("bridge1")
	if len(ports) != 0 {
		t.Errorf("expected empty ports, got %v", ports)
	}

	m.AddPort("bridge1", "ether1")
	m.AddPort("bridge1", "ether2")

	ports = m.GetBridgePorts("bridge1")
	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}

	// Test with non-existent bridge
	ports = m.GetBridgePorts("nonexistent")
	if ports != nil {
		t.Fatal("expected nil for non-existent bridge")
	}
}

// Test the SettingsDirectory interface
func TestSettingsAddAndList(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	// Add via SettingsDirectory interface
	_, err = m.Add(map[string]interface{}{
		"name": "bridge1",
		"mtu":  1400,
	})
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	_, err = m.Add(map[string]interface{}{
		"name": "bridge2",
	})
	if err != nil {
		t.Fatalf("failed to add bridge2: %v", err)
	}

	// List
	entries := m.List()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Check MTU was applied
	mtu, _ := entries[0].Property("mtu")
	if mtu != 1400 {
		t.Errorf("expected MTU 1400, got %v", mtu)
	}
}

func TestSettingsSetAndRemove(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	entry, err := m.Add(map[string]interface{}{
		"name": "bridge1",
	})
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	// Set via SettingsDirectory
	err = m.Set(entry.ID(), map[string]interface{}{
		"mtu": 1400,
	})
	if err != nil {
		t.Fatalf("failed to set MTU: %v", err)
	}

	mtu, _ := entry.Property("mtu")
	if mtu != 1400 {
		t.Errorf("expected MTU 1400, got %v", mtu)
	}

	// Remove
	err = m.Remove(entry.ID())
	if err != nil {
		t.Fatalf("failed to remove: %v", err)
	}

	_, exists := m.Get(entry.ID())
	if exists {
		t.Fatal("expected entry to be removed")
	}
}

func TestSettingsGet(t *testing.T) {
	m, err := New(nil)
	if err != nil {
		t.Fatalf("failed to create bridge module: %v", err)
	}

	entry, err := m.Add(map[string]interface{}{
		"name": "bridge1",
	})
	if err != nil {
		t.Fatalf("failed to add bridge: %v", err)
	}

	// Get by ID
	got, exists := m.Get(entry.ID())
	if !exists {
		t.Fatal("expected entry to exist")
	}

	gotName, _ := got.Property("name")
	if gotName != "bridge1" {
		t.Errorf("expected name 'bridge1', got %v", gotName)
	}

	// Get non-existent
	_, exists = m.Get("9999")
	if exists {
		t.Fatal("expected non-existent entry to return false")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"5s", 5 * time.Second},
		{"10m", 10 * time.Minute},
		{"1h", 1 * time.Hour},
		{"1d", 24 * time.Hour},
		{"5m", 5 * time.Minute},
	}

	for _, tt := range tests {
		d, err := parseDuration(tt.input)
		if err != nil {
			t.Errorf("parseDuration(%q) returned error: %v", tt.input, err)
			continue
		}
		if d != tt.expected {
			t.Errorf("parseDuration(%q) = %v, want %v", tt.input, d, tt.expected)
		}
	}
}

func TestParseDurationInvalid(t *testing.T) {
	_, err := parseDuration("invalid")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}

	_, err = parseDuration("")
	if err == nil {
		t.Fatal("expected error for empty duration")
	}
}

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"00:11:22:33:44:55", "00:11:22:33:44:55"},
		{"00-11-22-33-44-55", "00:11:22:33:44:55"},
		{"0011.2233.4455", "00:11:22:33:44:55"},
		{"FF:FF:FF:FF:FF:FF", "FF:FF:FF:FF:FF:FF"},
		{"ff:ff:ff:ff:ff:ff", "FF:FF:FF:FF:FF:FF"},
	}

	for _, tt := range tests {
		result := normalizeMAC(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeMAC(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
