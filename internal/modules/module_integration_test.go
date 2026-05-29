// Package modules_test contains cross-module integration tests that exercise
// the full CLI flow (parse → execute → module) for all registered modules,
// mirroring the production REPL initialisation in cmd/simulator/main.go.
package modules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	interfacesMod "github.com/Devaste/MikroLab/internal/modules/interface"
	ipAddr "github.com/Devaste/MikroLab/internal/modules/ip/address"
	"github.com/Devaste/MikroLab/internal/modules/ip/route"
	"github.com/Devaste/MikroLab/internal/tree"
)

// projectRoot attempts to find the project root by looking for go.mod.
func projectRoot() string {
	// Search upward from the test directory
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

// resolvePath resolves a path relative to the project root.
func resolvePath(relative string) string {
	return filepath.Join(projectRoot(), relative)
}

// setupFullTree builds the complete config tree with all real modules loaded
// from their JSON schema files, exactly matching the production REPL.
func setupFullTree(t *testing.T) {
	t.Helper()

	// 1. Create root /
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	// 2. Create /ip
	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	// 3. Load /interface schema and create the interface module.
	ifaceSchemaData, err := os.ReadFile(resolvePath("internal/modules/interface/schema.json"))
	if err != nil {
		t.Fatalf("failed to read interface schema: %v", err)
	}
	ifaceSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(ifaceSchemaData, ifaceSchema); err != nil {
		t.Fatalf("failed to parse interface schema: %v", err)
	}
	ifaceModule, err := interfacesMod.New(ifaceSchema)
	if err != nil {
		t.Fatalf("failed to create InterfaceModule: %v", err)
	}
	if err := tree.Root.AddChild("interface", ifaceModule); err != nil {
		t.Fatalf("failed to register /interface: %v", err)
	}

	// 4. Load IP route schema and create the route module.
	routeSchemaData, err := os.ReadFile(resolvePath("internal/modules/ip/route/schema.json"))
	if err != nil {
		t.Fatalf("failed to read route schema: %v", err)
	}
	routeSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(routeSchemaData, routeSchema); err != nil {
		t.Fatalf("failed to parse route schema: %v", err)
	}
	routeModule, err := route.New(routeSchema, ifaceModule)
	if err != nil {
		t.Fatalf("failed to create RouteModule: %v", err)
	}
	if err := ipDir.AddChild("route", routeModule); err != nil {
		t.Fatalf("failed to register /ip/route: %v", err)
	}

	// 5. Load IP address schema and create the address module.
	addrSchemaData, err := os.ReadFile(resolvePath("internal/modules/ip/address/schema.json"))
	if err != nil {
		t.Fatalf("failed to read IP address schema: %v", err)
	}
	addrSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(addrSchemaData, addrSchema); err != nil {
		t.Fatalf("failed to parse IP address schema: %v", err)
	}
	addrModule, err := ipAddr.New(addrSchema, ifaceModule, routeModule)
	if err != nil {
		t.Fatalf("failed to create IPAddressModule: %v", err)
	}
	if err := ipDir.AddChild("address", addrModule); err != nil {
		t.Fatalf("failed to register /ip/address: %v", err)
	}
}

// addInterface is a helper to create an interface entry via the CLI.
func addInterface(t *testing.T, name string) {
	t.Helper()
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "add",
		Params: map[string]string{
			"name": name,
		},
	})
	if err != nil {
		t.Fatalf("failed to add interface %q: %v", name, err)
	}
}

// ---------------------------------------------------------------------------
// /interface integration tests
// ---------------------------------------------------------------------------

func TestIntegrationInterfaceAddAndPrint(t *testing.T) {
	setupFullTree(t)

	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "add",
		Params: map[string]string{
			"name": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Print interfaces
	output, err = cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "print",
	})
	if err != nil {
		t.Fatalf("unexpected error on print: %v", err)
	}
	if len(output) == 0 {
		t.Fatal("expected print output, got empty")
	}
}

func TestIntegrationInterfaceAddDuplicateName(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "add",
		Params: map[string]string{
			"name": "ether1",
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate interface name, got nil")
	}
}

func TestIntegrationInterfaceSetAndPrint(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	// Set the MTU
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "set",
		Params: map[string]string{
			"numbers": "0",
			"mtu":     "9000",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	// Remove the interface
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on remove: %v", err)
	}
}

func TestIntegrationInterfaceDisableEnable(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	// Disable
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "disable",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on disable: %v", err)
	}

	// Enable
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "enable",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on enable: %v", err)
	}
}

func TestIntegrationInterfaceCannotRemoveMissing(t *testing.T) {
	setupFullTree(t)

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/interface",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err == nil {
		t.Fatal("expected error removing non-existent interface, got nil")
	}
}

// ---------------------------------------------------------------------------
// /ip/address integration tests
// ---------------------------------------------------------------------------

func TestIntegrationAddressAddAndPrint(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Print addresses
	output, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "print",
	})
	if err != nil {
		t.Fatalf("unexpected error on print: %v", err)
	}
	if len(output) == 0 {
		t.Fatal("expected print output, got empty")
	}
}

func TestIntegrationAddressAddMissingInterface(t *testing.T) {
	setupFullTree(t)

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "nonexistent",
		},
	})
	if err == nil {
		t.Fatal("expected error for non-existent interface, got nil")
	}
}

func TestIntegrationAddressAddReservedIP(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "127.0.0.1/8",
			"interface": "ether1",
		},
	})
	if err == nil {
		t.Fatal("expected error for reserved IP, got nil")
	}
}

func TestIntegrationAddressAddInvalidNetmask(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/33",
			"interface": "ether1",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid netmask, got nil")
	}
}

func TestIntegrationAddressSetAndRemove(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "10.0.0.1/8",
			"interface": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding address: %v", err)
	}

	// Set the comment
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "set",
		Params: map[string]string{
			"numbers": "0",
			"comment": "WAN interface",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	// Remove the address
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on remove: %v", err)
	}
}

func TestIntegrationAddressDuplicateOnSameInterface(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	// Add first address
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding first address: %v", err)
	}

	// Try duplicate
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether1",
		},
	})
	if err == nil {
		t.Fatal("expected error for duplicate IP on same interface, got nil")
	}
}

func TestIntegrationAddressSameIPDifferentInterface(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")
	addInterface(t, "ether2")

	// Add first address
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding first address: %v", err)
	}

	// Same IP on different interface should succeed
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether2",
		},
	})
	if err != nil {
		t.Fatalf("expected same IP on different interface to succeed, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Cross-module integration tests
// ---------------------------------------------------------------------------

func TestIntegrationAddressCreatesConnectedRoute(t *testing.T) {
	setupFullTree(t)

	addInterface(t, "ether1")

	// Add an IP address - this should automatically create a connected route
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error adding address: %v", err)
	}

	// Print routes to verify connected route was created
	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "print",
	})
	if err != nil {
		t.Fatalf("unexpected error on print routes: %v", err)
	}
	if len(output) == 0 {
		t.Fatal("expected routes to exist after adding IP address")
	}

	// Lookup an IP in the connected network
	lookupMod, ok := tree.GetNode("/ip/route").(*route.RouteModule)
	if !ok {
		t.Fatal("expected *RouteModule from tree")
	}
	result := lookupMod.Lookup("192.168.1.55")
	if result == nil {
		t.Fatal("expected lookup to find connected route")
	}
	if result.OutInterface != "ether1" {
		t.Errorf("expected outInterface 'ether1', got %q", result.OutInterface)
	}
	if result.Distance != 0 {
		t.Errorf("expected distance 0 for connected route, got %d", result.Distance)
	}

	// Remove the IP address - connected route should be removed
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error removing address: %v", err)
	}

	// Verify the connected route was removed
	result = lookupMod.Lookup("192.168.1.55")
	if result != nil {
		t.Fatal("expected lookup to return nil after address removal")
	}
}

func TestIntegrationFullWorkflow(t *testing.T) {
	setupFullTree(t)

	// 1. Create interfaces
	addInterface(t, "ether1")
	addInterface(t, "ether2")

	// 2. Add IP addresses
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "10.0.0.1/24",
			"interface": "ether1",
		},
	})
	if err != nil {
		t.Fatalf("failed to add address to ether1: %v", err)
	}

	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/address",
		Action: "add",
		Params: map[string]string{
			"address":   "192.168.1.1/24",
			"interface": "ether2",
		},
	})
	if err != nil {
		t.Fatalf("failed to add address to ether2: %v", err)
	}

	// 3. Add a static route
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "0.0.0.0/0",
			"gateway":     "10.0.0.254",
			"distance":    "1",
		},
	})
	if err != nil {
		t.Fatalf("failed to add default route: %v", err)
	}

	// 4. Print all tables
	for _, path := range []string{"/interface", "/ip/address", "/ip/route"} {
		output, err := cli.Execute(cli.ParsedCommand{
			Path:   path,
			Action: "print",
		})
		if err != nil {
			t.Fatalf("unexpected error printing %s: %v", path, err)
		}
		if len(output) == 0 {
			t.Fatalf("expected non-empty output for %s", path)
		}
	}

	// 5. Verify route lookup works end-to-end
	lookupMod, ok := tree.GetNode("/ip/route").(*route.RouteModule)
	if !ok {
		t.Fatal("expected *RouteModule from tree")
	}

	// Default route lookup
	result := lookupMod.Lookup("8.8.8.8")
	if result == nil {
		t.Fatal("expected lookup to find default route")
	}
	if result.Gateway != "10.0.0.254" {
		t.Errorf("expected gateway '10.0.0.254', got %q", result.Gateway)
	}
	if result.Distance != 1 {
		t.Errorf("expected distance 1, got %d", result.Distance)
	}

	// Connected route lookup
	result = lookupMod.Lookup("10.0.0.55")
	if result == nil {
		t.Fatal("expected lookup to find connected route")
	}
	if result.Distance != 0 {
		t.Errorf("expected distance 0 for connected route, got %d", result.Distance)
	}
}
