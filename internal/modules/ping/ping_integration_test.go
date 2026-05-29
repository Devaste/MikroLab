// Package ping provides integration tests for the ping command using the CLI.
//
// These tests set up the full configuration tree with /interface, /ip/route,
// and /ip/arp modules, then exercise the /ping command through the CLI.
package ping

import (
	"net"
	"strings"
	"testing"

	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/core"
	routeMod "github.com/Devaste/MikroLab/internal/modules/ip/route"
	"github.com/Devaste/MikroLab/internal/tree"
)

// TestPingToReachableIP tests ping to a directly connected reachable IP.
// It sets up: interface, ARP entry, and then pings through the CLI.
func TestPingToReachableIP(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	// Create interface module and add ether1
	ifaceMod := newMockInterfaceChecker(map[string]bool{"ether1": true})

	// Create route module with mock interfaces
	routeModInstance := newMockRouteModule("192.168.1.0/24", "ether1", 0)

	// Create ARP module with mock interface checker and add an entry
	arpModInstance := newMockARPModule(ifaceMod)
	arpModInstance.Add(map[string]interface{}{
		"address":     "192.168.1.2",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})

	// Create ping command
	pingCmd, err := New("/ping", "Ping", routeModInstance, arpModInstance)
	if err != nil {
		t.Fatalf("failed to create PingCommand: %v", err)
	}

	if err := tree.Root.AddChild("ping", pingCmd); err != nil {
		t.Fatalf("failed to register /ping: %v", err)
	}

	// Execute ping via CLI
	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ping",
		Action: "ping",
		Params: map[string]string{
			"address": "192.168.1.2",
			"count":   "3",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify output contains expected elements
	if !contains(output, "192.168.1.2") {
		t.Error("expected output to contain '192.168.1.2'")
	}
	if !contains(output, "sent=") {
		t.Error("expected output to contain 'sent='")
	}
	if !contains(output, "received=3") {
		t.Error("expected output to contain 'received=3'")
	}
	if !contains(output, "packet-loss=0%") {
		t.Error("expected output to contain 'packet-loss=0%'")
	}
}

// TestPingToUnreachableIP tests ping to an IP with no route.
func TestPingToUnreachableIP(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	// Create modules with empty route table
	ifaceMod := newMockInterfaceChecker(map[string]bool{"ether1": true})
	routeModInstance := newMockRouteModuleEmpty()
	arpModInstance := newMockARPModule(ifaceMod)

	pingCmd, err := New("/ping", "Ping", routeModInstance, arpModInstance)
	if err != nil {
		t.Fatalf("failed to create PingCommand: %v", err)
	}

	if err := tree.Root.AddChild("ping", pingCmd); err != nil {
		t.Fatalf("failed to register /ping: %v", err)
	}

	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ping",
		Action: "ping",
		Params: map[string]string{
			"address": "10.0.0.99",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show timeout and packet loss
	if !contains(output, "timeout") {
		t.Error("expected output to contain 'timeout'")
	}
	if !contains(output, "received=0") {
		t.Error("expected output to contain 'received=0'")
	}
	if !contains(output, "packet-loss=100%") {
		t.Error("expected output to contain 'packet-loss=100%'")
	}
}

// TestPingViaGateway tests ping to an IP reachable via a gateway (next-hop).
func TestPingViaGateway(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	ifaceMod := newMockInterfaceChecker(map[string]bool{"ether1": true})

	// Route: 192.168.2.0/24 via gateway 192.168.1.254 on ether1
	// Use a custom route module that returns OutInterface="ether1" so
	// the ping command can resolve the gateway MAC on the correct interface.
	routeModInstance := newMockRouteModuleWithIface("192.168.2.0/24", "192.168.1.254", 1, "ether1")

	// ARP: resolve the gateway IP on ether1
	arpModInstance := newMockARPModule(ifaceMod)
	arpModInstance.Add(map[string]interface{}{
		"address":     "192.168.1.254",
		"mac-address": "00:0C:42:11:22:33",
		"interface":   "ether1",
	})

	pingCmd, err := New("/ping", "Ping", routeModInstance, arpModInstance)
	if err != nil {
		t.Fatalf("failed to create PingCommand: %v", err)
	}

	if err := tree.Root.AddChild("ping", pingCmd); err != nil {
		t.Fatalf("failed to register /ping: %v", err)
	}

	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ping",
		Action: "ping",
		Params: map[string]string{
			"address": "192.168.2.1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(output, "192.168.2.1") {
		t.Error("expected output to contain '192.168.2.1'")
	}
	if !contains(output, "received=1") {
		t.Error("expected output to contain 'received=1'")
	}
}

// TestPingWithCount validates the count parameter.
func TestPingWithCount(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	ifaceMod := newMockInterfaceChecker(map[string]bool{"ether1": true})
	routeModInstance := newMockRouteModule("192.168.1.0/24", "ether1", 0)
	arpModInstance := newMockARPModule(ifaceMod)
	arpModInstance.Add(map[string]interface{}{
		"address":     "192.168.1.2",
		"mac-address": "64:D1:54:AA:BB:CC",
		"interface":   "ether1",
	})

	pingCmd, err := New("/ping", "Ping", routeModInstance, arpModInstance)
	if err != nil {
		t.Fatalf("failed to create PingCommand: %v", err)
	}

	if err := tree.Root.AddChild("ping", pingCmd); err != nil {
		t.Fatalf("failed to register /ping: %v", err)
	}

	// Ping with count=5
	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ping",
		Action: "ping",
		Params: map[string]string{
			"address": "192.168.1.2",
			"count":   "5",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(output, "received=5") {
		t.Error("expected output to contain 'received=5'")
	}
}

// TestPingMissingAddress tests that missing address produces an error.
func TestPingMissingAddress(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ifaceMod := newMockInterfaceChecker(map[string]bool{"ether1": true})
	routeModInstance := newMockRouteModuleEmpty()
	arpModInstance := newMockARPModule(ifaceMod)

	pingCmd, err := New("/ping", "Ping", routeModInstance, arpModInstance)
	if err != nil {
		t.Fatalf("failed to create PingCommand: %v", err)
	}

	if err := tree.Root.AddChild("ping", pingCmd); err != nil {
		t.Fatalf("failed to register /ping: %v", err)
	}

	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ping",
		Action: "ping",
		Params: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error for missing address")
	}
}

// ---------------------------------------------------------------------------
// Mock implementations for testing
// ---------------------------------------------------------------------------

// mockInterfaceChecker implements the interface checker for testing.
type mockInterfaceChecker struct {
	interfaces map[string]bool
}

func newMockInterfaceChecker(ifaces map[string]bool) *mockInterfaceChecker {
	return &mockInterfaceChecker{interfaces: ifaces}
}

func (m *mockInterfaceChecker) InterfaceExists(name string) bool {
	return m.interfaces[name]
}

func (m *mockInterfaceChecker) ListInterfaces() []string {
	names := make([]string, 0, len(m.interfaces))
	for n := range m.interfaces {
		names = append(names, n)
	}
	return names
}

// mockRouteModule provides a simple route lookup for testing.
type mockRouteModule struct {
	dstNetwork string
	gateway    string
	distance   int
	hasRoutes  bool
}

func newMockRouteModule(dstNetwork, gateway string, distance int) *mockRouteModule {
	return &mockRouteModule{
		dstNetwork: dstNetwork,
		gateway:    gateway,
		distance:   distance,
		hasRoutes:  true,
	}
}

func newMockRouteModuleEmpty() *mockRouteModule {
	return &mockRouteModule{hasRoutes: false}
}

// newMockRouteModuleWithIface creates a route module that returns a specific outInterface.
type mockRouteModuleWithIface struct {
	dstNetwork string
	gateway    string
	distance   int
	outIface   string
	hasRoutes  bool
}

func newMockRouteModuleWithIface(dstNetwork, gateway string, distance int, outIface string) *mockRouteModuleWithIface {
	return &mockRouteModuleWithIface{
		dstNetwork: dstNetwork,
		gateway:    gateway,
		distance:   distance,
		outIface:   outIface,
		hasRoutes:  true,
	}
}

func (m *mockRouteModuleWithIface) Lookup(dstIP string) *routeMod.LookupResult {
	if !m.hasRoutes {
		return nil
	}
	if isIPInNetwork(dstIP, m.dstNetwork) {
		return &routeMod.LookupResult{
			Gateway:      m.gateway,
			OutInterface: m.outIface,
			Distance:     m.distance,
		}
	}
	return nil
}

// isIPInNetwork checks if an IP address falls within a CIDR network.
func isIPInNetwork(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return network.Contains(ip)
}

func (m *mockRouteModule) Lookup(dstIP string) *routeMod.LookupResult {
	if !m.hasRoutes {
		return nil
	}
	if isIPInNetwork(dstIP, m.dstNetwork) {
		outIface := ""
		if !strings.Contains(m.gateway, ".") && !strings.Contains(m.gateway, ":") {
			// Gateway is an interface name
			outIface = m.gateway
		}
		return &routeMod.LookupResult{
			Gateway:      m.gateway,
			OutInterface: outIface,
			Distance:     m.distance,
		}
	}
	return nil
}

// mockARPModule provides a simple ARP resolution for testing.
type mockARPModule struct {
	entries  map[string]string // "ip|interface" -> mac
	ifaceMod interface{ InterfaceExists(name string) bool }
}

func newMockARPModule(ifaceMod interface{ InterfaceExists(name string) bool }) *mockARPModule {
	return &mockARPModule{
		entries:  make(map[string]string),
		ifaceMod: ifaceMod,
	}
}

func (m *mockARPModule) Add(props map[string]interface{}) error {
	ip, _ := props["address"].(string)
	mac, _ := props["mac-address"].(string)
	iface, _ := props["interface"].(string)
	key := ip + "|" + iface
	m.entries[key] = mac
	return nil
}

func (m *mockARPModule) Resolve(ip string, ifaceName string) (string, bool) {
	key := ip + "|" + ifaceName
	mac, ok := m.entries[key]
	return mac, ok
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
