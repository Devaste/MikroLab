package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	interfaceMod "github.com/Devaste/MikroLab/internal/modules/interface"
	ipAddr "github.com/Devaste/MikroLab/internal/modules/ip/address"
	routeMod "github.com/Devaste/MikroLab/internal/modules/ip/route"
	"github.com/Devaste/MikroLab/internal/tree"
)

// ---------------------------------------------------------------------------
// Module loader
// ---------------------------------------------------------------------------

// registerModules builds the tree and registers all known modules.
func registerModules() (map[string]core.Node, error) {
	modules := make(map[string]core.Node)

	// 1. Create root /
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")

	// 2. Create /ip
	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := tree.Root.AddChild("ip", ipDir); err != nil {
		return nil, fmt.Errorf("failed to add /ip: %w", err)
	}

	// 3. Load the /interface schema and create the interface module.
	ifaceSchemaData, err := os.ReadFile("internal/modules/interface/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read interface schema: %w", err)
	}
	ifaceSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(ifaceSchemaData, ifaceSchema); err != nil {
		return nil, fmt.Errorf("failed to parse interface schema: %w", err)
	}

	ifaceModule, err := interfaceMod.New(ifaceSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to create InterfaceModule: %w", err)
	}

	// Register /interface under root
	if err := tree.Root.AddChild("interface", ifaceModule); err != nil {
		return nil, fmt.Errorf("failed to register /interface: %w", err)
	}

	// 4. Load the IP route schema from the JSON file and create the route module.
	routeSchemaData, err := os.ReadFile("internal/modules/ip/route/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read route schema: %w", err)
	}
	routeSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(routeSchemaData, routeSchema); err != nil {
		return nil, fmt.Errorf("failed to parse route schema: %w", err)
	}

	routeModule, err := routeMod.New(routeSchema, ifaceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to create RouteModule: %w", err)
	}

	if err := ipDir.AddChild("route", routeModule); err != nil {
		return nil, fmt.Errorf("failed to register /ip/route: %w", err)
	}

	// 5. Load the IP address schema from the JSON file.
	schemaData, err := os.ReadFile("internal/modules/ip/address/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read IP address schema: %w", err)
	}
	schema := &config.ModuleSchema{}
	if err := json.Unmarshal(schemaData, schema); err != nil {
		return nil, fmt.Errorf("failed to parse IP address schema: %w", err)
	}

	ipAddrModule, err := ipAddr.New(schema, ifaceModule, routeModule)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPAddressModule: %w", err)
	}

	if err := ipDir.AddChild("address", ipAddrModule); err != nil {
		return nil, fmt.Errorf("failed to register /ip/address: %w", err)
	}

	// 6. Register modules in the lookup map
	modules["/interface"] = ifaceModule
	modules["/ip/route"] = routeModule
	modules["/ip/address"] = ipAddrModule
	return modules, nil
}

// ---------------------------------------------------------------------------
// REPL loop
// ---------------------------------------------------------------------------

// runREPL starts an interactive RouterOS-style command loop.
func runREPL() {
	scanner := bufio.NewScanner(os.Stdin)

	// Set up a channel to catch Ctrl+C (SIGINT)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine to handle interrupt — when we get SIGINT, print a newline
	// and return, breaking the REPL.
	go func() {
		<-sigCh
		fmt.Println()
		fmt.Println("Interrupted. Use 'exit' or 'quit' to exit.")
		// We don't os.Exit here — the main loop will continue on next Scan()
		// but bufio.Scanner won't recover from a partial read on Windows.
		// For simplicity, we just print a message and let the user type exit.
	}()

	fmt.Println("MikroLab RouterOS Simulator v0.1")
	fmt.Println("Type 'exit' or 'quit' to exit.")
	fmt.Println()

	for {
		fmt.Print("[admin@MikroLab] > ")

		if !scanner.Scan() {
			// EOF or error — break out
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for exit/quit
		if line == "exit" || line == "quit" {
			fmt.Println("Exiting.")
			break
		}

		// Parse the command
		parsed, err := cli.Parse(line)
		if err != nil {
			fmt.Printf("failure: %v\n", err)
			continue
		}

		// Execute the command
		output, err := cli.Execute(parsed)
		if err != nil {
			fmt.Printf("%v\n", err)
			continue
		}

		fmt.Print(output)
	}
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	modules, err := registerModules()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Println("=== MikroLab Configuration Tree ===")
	fmt.Println()

	// Print all registered paths
	for path, module := range modules {
		fmt.Printf("  %-20s  [%s]  %s\n", path, module.Type(), module.Title())
	}

	fmt.Println()
	fmt.Println("--- Tree Traversal Tests ---")

	// Verify traversal through tree.GetNode
	testPaths := []string{
		"/",
		"/ip",
		"/ip/address",
		"/ip/address/nonexistent",
		"relative",
	}

	for _, p := range testPaths {
		node := tree.GetNode(p)
		if node != nil {
			fmt.Printf("  %-30s → %s (%s)\n", p, node.Path(), node.Type())
		} else {
			fmt.Printf("  %-30s → <nil>\n", p)
		}
	}

	fmt.Println("=== Simulator initialised ===")
	fmt.Println()

	// Start the REPL
	runREPL()
}
