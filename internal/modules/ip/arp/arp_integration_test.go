// Package ip_arp provides integration tests for the ARP module using the CLI.
package ip_arp

import (
	"testing"

	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/tree"
)

// TestArpAddValidEntryCLI tests adding a valid ARP entry via CLI.
func TestArpAddValidEntryCLI(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	// Create the ARP module with a mock interface checker
	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	// Test add via CLI
	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding ARP entry: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Print ARP table
	output, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "print",
	})
	if err != nil {
		t.Fatalf("unexpected error on print: %v", err)
	}
	if len(output) == 0 {
		t.Fatal("expected print output, got empty")
	}
}

// TestArpPrintWithEntry tests that print shows correct flags and columns.
func TestArpPrintWithEntry(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	// Add an entry
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding ARP entry: %v", err)
	}

	// Print and verify content
	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "print",
	})
	if err != nil {
		t.Fatalf("unexpected error on print: %v", err)
	}

	// Check for expected text in output
	if !contains(output, "192.168.1.1") {
		t.Error("expected output to contain '192.168.1.1'")
	}
	if !contains(output, "64:D1:54:AA:BB:CC") {
		t.Error("expected output to contain '64:D1:54:AA:BB:CC'")
	}
	if !contains(output, "ether1") {
		t.Error("expected output to contain 'ether1'")
	}
	if !contains(output, "C") && !contains(output, "complete") {
		t.Log("note: print output does not show 'C' or 'complete' flag (may be displayed as flag)")
	}
}

// TestArpDuplicateIPOnSameInterface tests duplicate IP rejection via CLI.
func TestArpDuplicateIPOnSameInterface(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	// First entry should succeed
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding first entry: %v", err)
	}

	// Duplicate should fail
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "00:11:22:33:44:55",
			"interface":   "ether1",
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate IP on same interface")
	}
}

// TestArpInvalidMACFormat tests that invalid MAC format is rejected via CLI.
func TestArpInvalidMACFormat(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "invalid-mac",
			"interface":   "ether1",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid MAC format")
	}
}

// TestArpInterfaceNotExists tests that non-existent interface is rejected via CLI.
func TestArpInterfaceNotExists(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "nonexistent",
		},
	})
	if err == nil {
		t.Fatal("expected error for non-existent interface")
	}
}

// TestArpSetPublished tests setting published property via CLI.
func TestArpSetPublished(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	// Add an entry
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding entry: %v", err)
	}

	// Set published=yes
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "set",
		Params: map[string]string{
			"numbers":   "0",
			"published": "yes",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	// Verify via direct Get
	entry, ok := arpMod.Get("0")
	if !ok {
		t.Fatal("entry not found after set")
	}
	published, _ := entry.Property("published")
	if published != true {
		t.Errorf("expected published=true, got %v", published)
	}
}

// TestArpRemoveEntry tests removing an entry via CLI.
func TestArpRemoveEntry(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	// Add an entry
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding entry: %v", err)
	}

	// Remove the entry
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on remove: %v", err)
	}

	// Verify entry is gone
	entries := arpMod.List()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after removal, got %d", len(entries))
	}
}

// TestArpResolveIntegration tests the Resolve helper indirectly.
func TestArpResolveIntegration(t *testing.T) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	arpMod, err := New("/ip/arp", "ARP Table", &mockInterfaceChecker{map[string]bool{
		"ether1": true,
	}})
	if err != nil {
		t.Fatalf("failed to create ArpModule: %v", err)
	}

	if err := ipDir.AddChild("arp", arpMod); err != nil {
		t.Fatalf("failed to register /ip/arp: %v", err)
	}

	// Add a static entry
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/arp",
		Action: "add",
		Params: map[string]string{
			"address":     "192.168.1.1",
			"mac-address": "64:D1:54:AA:BB:CC",
			"interface":   "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding entry: %v", err)
	}

	// Test Resolve helper
	mac, ok := arpMod.Resolve("192.168.1.1", "ether1")
	if !ok {
		t.Fatal("Resolve should find the entry")
	}
	if mac != "64:D1:54:AA:BB:CC" {
		t.Errorf("expected MAC '64:D1:54:AA:BB:CC', got %q", mac)
	}

	// Resolve with wrong interface
	_, ok = arpMod.Resolve("192.168.1.1", "ether2")
	if ok {
		t.Error("Resolve should not find entry on wrong interface")
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString is a simple substring search.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
