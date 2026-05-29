package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Devaste/MikroLab/internal/api"
	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	interfaceMod "github.com/Devaste/MikroLab/internal/modules/interface"
	bridgeMod "github.com/Devaste/MikroLab/internal/modules/interface/bridge"
	bridgePortMod "github.com/Devaste/MikroLab/internal/modules/interface/bridge/port"
	ipAddr "github.com/Devaste/MikroLab/internal/modules/ip/address"
	arpMod "github.com/Devaste/MikroLab/internal/modules/ip/arp"
	firewallFilter "github.com/Devaste/MikroLab/internal/modules/ip/firewall/filter"
	routeMod "github.com/Devaste/MikroLab/internal/modules/ip/route"
	logMod "github.com/Devaste/MikroLab/internal/modules/log"
	pingMod "github.com/Devaste/MikroLab/internal/modules/ping"
	"github.com/Devaste/MikroLab/internal/topology"
	"github.com/Devaste/MikroLab/internal/tree"
)

// ---------------------------------------------------------------------------
// Module loader
// ---------------------------------------------------------------------------

// registerModulesOnTree builds the tree and registers all known modules
// on the given tree root.
func registerModulesOnTree(root *tree.TreeNode) (map[string]core.Node, error) {
	modules := make(map[string]core.Node)

	// 1. Create /ip
	ipDir := tree.NewTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := root.AddChild("ip", ipDir); err != nil {
		return nil, fmt.Errorf("failed to add /ip: %w", err)
	}

	// 2. Load the /interface schema and create the interface module.
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
	if err := root.AddChild("interface", ifaceModule); err != nil {
		return nil, fmt.Errorf("failed to register /interface: %w", err)
	}

	// 3. Load the IP route schema from the JSON file and create the route module.
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

	// 4. Load the IP address schema from the JSON file.
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

	// 5. Load the ARP schema (optional, used for metadata only) and create module.
	arpModInstance, err := arpMod.New("/ip/arp", "ARP Table", ifaceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to create ArpModule: %w", err)
	}

	if err := ipDir.AddChild("arp", arpModInstance); err != nil {
		return nil, fmt.Errorf("failed to register /ip/arp: %w", err)
	}

	// 6. Load bridge schema and create the bridge module.
	bridgeSchemaData, err := os.ReadFile("internal/modules/interface/bridge/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read bridge schema: %w", err)
	}
	bridgeSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(bridgeSchemaData, bridgeSchema); err != nil {
		return nil, fmt.Errorf("failed to parse bridge schema: %w", err)
	}

	bridgeModule, err := bridgeMod.New(bridgeSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to create BridgeModule: %w", err)
	}

	// Register as a child of /interface (RoS uses /interface bridge)
	ifaceDir, _ := root.Child("interface")
	ifaceNode, ok := ifaceDir.(core.Directory)
	if !ok {
		return nil, fmt.Errorf("/interface is not a Directory")
	}
	if err := ifaceNode.AddChild("bridge", bridgeModule); err != nil {
		return nil, fmt.Errorf("failed to register /interface/bridge: %w", err)
	}

	// 7. Load bridge port schema and create the bridge port module.
	bridgePortSchemaData, err := os.ReadFile("internal/modules/interface/bridge/port/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read bridge port schema: %w", err)
	}
	bridgePortSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(bridgePortSchemaData, bridgePortSchema); err != nil {
		return nil, fmt.Errorf("failed to parse bridge port schema: %w", err)
	}

	bridgePortModule, err := bridgePortMod.New(
		bridgePortSchema,
		ifaceModule,
		bridgeModule,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create BridgePortModule: %w", err)
	}

	// Register as a child of /interface (sibling of bridge, since bridge is a list node)
	if err := ifaceNode.AddChild("bridge_port", bridgePortModule); err != nil {
		return nil, fmt.Errorf("failed to register /interface/bridge_port: %w", err)
	}

	// 8. Create the ping command (registers under root, not /ip).
	pingCmd, err := pingMod.New("/ping", "Ping", routeModule, arpModInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to create PingCommand: %w", err)
	}

	if err := root.AddChild("ping", pingCmd); err != nil {
		return nil, fmt.Errorf("failed to register /ping: %w", err)
	}

	// 9. Create /ip/firewall directory
	ipFirewallDir := tree.NewTreeNode("/ip/firewall", core.NodeTypeDirectory, "IP Firewall")
	if err := ipDir.AddChild("firewall", ipFirewallDir); err != nil {
		return nil, fmt.Errorf("failed to add /ip/firewall: %w", err)
	}

	// 10. Load firewall filter schema and create the filter module.
	filterSchemaData, err := os.ReadFile("internal/modules/ip/firewall/filter/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read filter schema: %w", err)
	}
	filterSchema := &config.ModuleSchema{}
	if err := json.Unmarshal(filterSchemaData, filterSchema); err != nil {
		return nil, fmt.Errorf("failed to parse filter schema: %w", err)
	}

	// The log module is created later and set on the filter module
	filterModule, err := firewallFilter.New(filterSchema, ifaceModule, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create FilterModule: %w", err)
	}

	if err := ipFirewallDir.AddChild("filter", filterModule); err != nil {
		return nil, fmt.Errorf("failed to register /ip/firewall/filter: %w", err)
	}

	// 11. Create the log module
	logModule, err := logMod.New("/log", "System Log")
	if err != nil {
		return nil, fmt.Errorf("failed to create LogModule: %w", err)
	}

	if err := root.AddChild("log", logModule); err != nil {
		return nil, fmt.Errorf("failed to register /log: %w", err)
	}

	// 12. Wire up log module to filter module for log action support
	if err := filterModule.SetLogAdder(logModule); err != nil {
		return nil, fmt.Errorf("failed to set log adder on filter module: %w", err)
	}

	// 13. Register modules in the lookup map
	modules["/interface"] = ifaceModule
	modules["/interface/bridge"] = bridgeModule
	modules["/interface/bridge/port"] = bridgePortModule
	modules["/interface/bridge_port"] = bridgePortModule
	modules["/ip/route"] = routeModule
	modules["/ip/address"] = ipAddrModule
	modules["/ip/arp"] = arpModInstance
	modules["/ip/firewall/filter"] = filterModule
	modules["/log"] = logModule
	modules["/ping"] = pingCmd
	return modules, nil
}

// registerModules builds the default tree and registers all known modules.
// This is kept for backward compatibility.
func registerModules() (map[string]core.Node, error) {
	tree.Root = tree.NewTreeNode("/", core.NodeTypeDirectory, "root")
	return registerModulesOnTree(tree.Root)
}

// populateDeviceTree registers modules on a device's tree.
func populateDeviceTree(dev *topology.Device) error {
	_, err := registerModulesOnTree(dev.Tree)
	return err
}

// ---------------------------------------------------------------------------
// REPL loop
// ---------------------------------------------------------------------------

// runREPL starts an interactive RouterOS-style command loop.
func runREPL(topo *topology.Topology) {
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
	// Create the topology manager
	topo := topology.NewTopology()

	// Create the default device "Router1"
	defaultDevice, err := topo.CreateDeviceWithID("device-1", "Router1")
	if err != nil {
		fmt.Printf("ERROR: failed to create default device: %v\n", err)
		return
	}

	// Populate the default device's tree with modules
	if err := populateDeviceTree(defaultDevice); err != nil {
		fmt.Printf("ERROR: failed to populate default device tree: %v\n", err)
		return
	}

	// Set the global tree root for backward compatibility (REPL and existing code)
	tree.Root = defaultDevice.Tree

	modules, err := registerModulesOnTree(tree.Root)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	// Set up bridge handler on topology
	if bridgeNode, ok := modules["/interface/bridge"]; ok {
		if bridgeHandler, ok := bridgeNode.(topology.BridgeHandler); ok {
			topo.SetBridgeHandler(bridgeHandler)
			fmt.Println("Bridge handler registered on topology manager")
		}
	}

	// Set up firewall evaluator on topology
	if filterNode, ok := modules["/ip/firewall/filter"]; ok {
		if fwEvaluator, ok := filterNode.(topology.FirewallEvaluator); ok {
			topo.SetFirewallEvaluator(fwEvaluator)
			fmt.Println("Firewall evaluator registered on topology manager")
		}
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

	// Start the WebSocket API server in a goroutine
	wsAddr := ":8080"
	if envAddr := os.Getenv("MIKROLAB_WS_ADDR"); envAddr != "" {
		wsAddr = envAddr
	}

	wsServer, err := api.NewServer(wsAddr)
	if err != nil {
		fmt.Printf("ERROR: failed to create WebSocket server: %v\n", err)
		return
	}

	// Set the topology on the server (replaces the empty one created by NewServer)
	wsServer.SetTopology(topo)

	go func() {
		fmt.Printf("WebSocket API server starting on %s (ws://localhost%s/ws)\n", wsAddr, wsAddr)
		if err := wsServer.Start(); err != nil {
			fmt.Printf("WebSocket server error: %v\n", err)
		}
	}()

	// Start the REPL
	runREPL(topo)

	// Graceful shutdown
	fmt.Println("Shutting down...")
	wsServer.Stop()
}
