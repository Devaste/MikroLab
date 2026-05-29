package route_test

import (
	"testing"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/modules/ip/route"
)

// mockInterfaceChecker implements route.InterfaceChecker for tests.
type mockInterfaceChecker struct{}

func (m *mockInterfaceChecker) InterfaceExists(name string) bool {
	return name == "ether1" || name == "ether2"
}

func (m *mockInterfaceChecker) ListInterfaces() []string {
	return []string{"ether1", "ether2"}
}

func TestRouteAddAndList(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
			"distance":    {Name: "distance", Type: config.SchemaInteger, Default: 1},
		},
		Actions: map[string]*config.SchemaAction{
			"add": {Name: "add", Parameters: []string{"dst-address", "gateway", "distance"}},
		},
		Defaults: map[string]interface{}{"distance": 1},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	entry, err := mod.Add(map[string]interface{}{
		"dst-address": "10.0.0.0/24",
		"gateway":     "192.168.1.2",
		"distance":    1,
	})
	if err != nil {
		t.Fatalf("unexpected error adding route: %v", err)
	}

	if entry.Dynamic() {
		t.Error("expected static route, got dynamic")
	}
	if entry.Disabled() {
		t.Error("expected active route, got disabled")
	}

	entries := mod.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	dst, _ := entries[0].Property("dst-address")
	if dst != "10.0.0.0/24" {
		t.Errorf("expected dst-address '10.0.0.0/24', got %v", dst)
	}
	gw, _ := entries[0].Property("gateway")
	if gw != "192.168.1.2" {
		t.Errorf("expected gateway '192.168.1.2', got %v", gw)
	}
}

func TestRouteAddConnectedRoute(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
			"distance":    {Name: "distance", Type: config.SchemaInteger, Default: 1},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	// Add a connected route (simulating what /ip/address would do)
	mod.AddConnectedRoute("192.168.1.0/24", "ether1")

	entries := mod.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if !e.Dynamic() {
		t.Error("expected connected route to be dynamic")
	}

	flags := e.Flags()
	if !flags["connect"] {
		t.Error("expected connected route to have connect flag")
	}
	if !flags["active"] {
		t.Error("expected connected route to be active")
	}

	dst, _ := e.Property("dst-address")
	if dst != "192.168.1.0/24" {
		t.Errorf("expected dst-address '192.168.1.0/24', got %v", dst)
	}
}

func TestRouteCannotRemoveDynamic(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	mod.AddConnectedRoute("192.168.1.0/24", "ether1")
	entries := mod.List()

	err = mod.Remove(entries[0].ID())
	if err == nil {
		t.Error("expected error removing dynamic route, got nil")
	}
}

func TestRouteGatewayInterfaceName(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	// Adding a route with interface name as gateway
	_, err = mod.Add(map[string]interface{}{
		"dst-address": "10.0.0.0/24",
		"gateway":     "ether1",
	})
	if err != nil {
		t.Fatalf("expected valid gateway (interface name), got error: %v", err)
	}
}

func TestRouteInvalidGateway(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	// Invalid gateway - non-existent interface
	_, err = mod.Add(map[string]interface{}{
		"dst-address": "10.0.0.0/24",
		"gateway":     "nonexistent",
	})
	if err == nil {
		t.Error("expected error for invalid gateway, got nil")
	}
}

func TestRouteInvalidCIDR(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	_, err = mod.Add(map[string]interface{}{
		"dst-address": "invalid-cidr",
		"gateway":     "192.168.1.1",
	})
	if err == nil {
		t.Error("expected error for invalid CIDR, got nil")
	}
}

func TestRouteLookup(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	// Add some routes
	mod.AddConnectedRoute("10.0.0.0/24", "ether1")
	mod.AddConnectedRoute("0.0.0.0/0", "ether2")

	// Lookup an IP that matches the connected route
	result := mod.Lookup("10.0.0.55")
	if result == nil {
		t.Fatal("expected lookup result, got nil")
	}
	if result.OutInterface != "ether1" {
		t.Errorf("expected outInterface 'ether1', got %q", result.OutInterface)
	}

	// Lookup an IP that should match the default route
	result = mod.Lookup("8.8.8.8")
	if result == nil {
		t.Fatal("expected lookup result for default route, got nil")
	}
}

func TestRemoveConnectedRoute(t *testing.T) {
	schema := &config.ModuleSchema{
		Path:  "/ip/route",
		Type:  "list",
		Title: "Routes",
		Schema: map[string]*config.SchemaProperty{
			"dst-address": {Name: "dst-address", Type: config.SchemaString, Required: true},
			"gateway":     {Name: "gateway", Type: config.SchemaString, Required: true},
		},
	}

	mod, err := route.New(schema, &mockInterfaceChecker{})
	if err != nil {
		t.Fatalf("failed to create route module: %v", err)
	}

	mod.AddConnectedRoute("192.168.1.0/24", "ether1")
	mod.AddConnectedRoute("10.0.0.0/24", "ether1")

	entries := mod.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	mod.RemoveConnectedRoute("192.168.1.0/24")

	entries = mod.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after removal, got %d", len(entries))
	}

	dst, _ := entries[0].Property("dst-address")
	if dst != "10.0.0.0/24" {
		t.Errorf("expected remaining dst-address '10.0.0.0/24', got %v", dst)
	}
}
