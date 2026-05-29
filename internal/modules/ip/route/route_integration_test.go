// Package route_test contains integration tests that exercise the full CLI
// flow (parse → execute → module) for /ip/route.
package route_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	interfacesMod "github.com/Devaste/MikroLab/internal/modules/interface"
	"github.com/Devaste/MikroLab/internal/modules/ip/route"
	"github.com/Devaste/MikroLab/internal/tree"
)

// setupIntegration builds the full config tree with real modules loaded from
// their JSON schema files, mirroring the production REPL initialisation in
// cmd/simulator/main.go.
func setupIntegration(t *testing.T) {
	t.Helper()

	// 1. Create root /
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	// 2. Create /ip
	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		t.Fatalf("failed to add /ip: %v", err)
	}

	// 3. Load /interface schema and create the interface module.
	ifaceSchemaData, err := os.ReadFile("../../interface/schema.json")
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

	// Pre-create an interface for gateway tests
	_, err = ifaceModule.Add(map[string]interface{}{
		"name": "ether1",
		"type": "ether",
	})
	if err != nil {
		t.Fatalf("failed to add ether1 interface: %v", err)
	}
	_, err = ifaceModule.Add(map[string]interface{}{
		"name": "ether2",
		"type": "ether",
	})
	if err != nil {
		t.Fatalf("failed to add ether2 interface: %v", err)
	}

	// 4. Load route schema.
	routeSchemaData, err := os.ReadFile("schema.json")
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
}

// ---------------------------------------------------------------------------
// Integration tests – full CLI flow
// ---------------------------------------------------------------------------

func TestIntegrationRouteAddAndPrint(t *testing.T) {
	setupIntegration(t)

	// Add a route via CLI
	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "10.0.0.0/24",
			"gateway":     "192.168.1.1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	// Print routes
	output, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "print",
	})
	if err != nil {
		t.Fatalf("unexpected error on print: %v", err)
	}
	if len(output) == 0 {
		t.Fatal("expected print output, got empty")
	}
}

func TestIntegrationRouteAddInvalidCIDR(t *testing.T) {
	setupIntegration(t)

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "not-a-cidr",
			"gateway":     "192.168.1.1",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}

func TestIntegrationRouteAddInvalidGateway(t *testing.T) {
	setupIntegration(t)

	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "10.0.0.0/24",
			"gateway":     "nonexistent",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid gateway, got nil")
	}
}

func TestIntegrationRouteAddGatewayInterfaceName(t *testing.T) {
	setupIntegration(t)

	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "10.0.0.0/24",
			"gateway":     "ether1",
		},
	})
	if err != nil {
		t.Fatalf("expected valid gateway (interface name), got error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestIntegrationRouteAddDefaultRoute(t *testing.T) {
	setupIntegration(t)

	output, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "0.0.0.0/0",
			"gateway":     "192.168.1.1",
		},
	})
	if err != nil {
		t.Fatalf("expected valid default route, got error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestIntegrationRouteSetAndRemove(t *testing.T) {
	setupIntegration(t)

	// Add a route
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "10.0.0.0/24",
			"gateway":     "192.168.1.1",
			"distance":    "5",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Set a property on the route
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "set",
		Params: map[string]string{
			"numbers":  "0",
			"distance": "10",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on set: %v", err)
	}

	// Remove the route
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on remove: %v", err)
	}
}

func TestIntegrationRouteCannotSetDynamic(t *testing.T) {
	setupIntegration(t)

	// The route module is setup without address integration so we call
	// AddConnectedRoute directly to create a dynamic route.
	// We need access to the module instance — re-retrieve it from tree.
	node := tree.GetNode("/ip/route")
	if node == nil {
		t.Fatal("route module not found in tree")
	}
	routeMod, ok := node.(*route.RouteModule)
	if !ok {
		t.Fatal("expected *RouteModule from tree")
	}

	// Add a dynamic connected route
	routeMod.AddConnectedRoute("192.168.10.0/24", "ether1")

	// Try to set a property on the dynamic route (index 0)
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "set",
		Params: map[string]string{
			"numbers":  "0",
			"distance": "10",
		},
	})
	if err == nil {
		t.Fatal("expected error when setting dynamic route, got nil")
	}
}

func TestIntegrationRouteCannotRemoveDynamic(t *testing.T) {
	setupIntegration(t)

	node := tree.GetNode("/ip/route")
	if node == nil {
		t.Fatal("route module not found in tree")
	}
	routeMod, ok := node.(*route.RouteModule)
	if !ok {
		t.Fatal("expected *RouteModule from tree")
	}

	// Add a dynamic connected route
	routeMod.AddConnectedRoute("192.168.20.0/24", "ether1")

	// Try to remove the dynamic route (index 0)
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "remove",
		Params: map[string]string{
			"numbers": "0",
		},
	})
	if err == nil {
		t.Fatal("expected error when removing dynamic route, got nil")
	}
}

func TestIntegrationRouteSetWithInvalidGateway(t *testing.T) {
	setupIntegration(t)

	// Add a route with a valid gateway
	_, err := cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "add",
		Params: map[string]string{
			"dst-address": "10.0.0.0/24",
			"gateway":     "192.168.1.1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to change the gateway to an invalid interface name
	_, err = cli.Execute(cli.ParsedCommand{
		Path:   "/ip/route",
		Action: "set",
		Params: map[string]string{
			"numbers": "0",
			"gateway": "nonexistent",
		},
	})
	if err == nil {
		t.Fatal("expected error when setting invalid gateway, got nil")
	}
}
