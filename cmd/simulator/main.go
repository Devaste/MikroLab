package main

import (
	"fmt"
	"strings"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/modules/interfaces"
	ipAddr "github.com/Devaste/MikroLab/internal/modules/ip_address"
)

// ---------------------------------------------------------------------------
// Concrete treeNode: implements core.Directory
// ---------------------------------------------------------------------------

// treeNode is a concrete directory node that implements core.Directory.
// It stores child nodes in a map and satisfies all interfaces needed to
// build the configuration tree.
type treeNode struct {
	path     string
	nodeType core.NodeType
	title    string
	children map[string]core.Node
}

func newTreeNode(path string, nodeType core.NodeType, title string) *treeNode {
	return &treeNode{
		path:     path,
		nodeType: nodeType,
		title:    title,
		children: make(map[string]core.Node),
	}
}

// core.Node interface
func (n *treeNode) Path() string        { return n.path }
func (n *treeNode) Type() core.NodeType { return n.nodeType }
func (n *treeNode) Title() string       { return n.title }

// core.Directory interface
func (n *treeNode) Children() map[string]core.Node { return n.children }
func (n *treeNode) Child(name string) (core.Node, bool) {
	child, ok := n.children[name]
	return child, ok
}
func (n *treeNode) AddChild(name string, child core.Node) error {
	if _, exists := n.children[name]; exists {
		return fmt.Errorf("child %q already exists under %q", name, n.path)
	}
	n.children[name] = child
	return nil
}
func (n *treeNode) RemoveChild(name string) error {
	if _, exists := n.children[name]; !exists {
		return fmt.Errorf("child %q not found under %q", name, n.path)
	}
	delete(n.children, name)
	return nil
}

// ---------------------------------------------------------------------------
// Compile-time checks
// ---------------------------------------------------------------------------
var _ core.Node = (*treeNode)(nil)
var _ core.Directory = (*treeNode)(nil)

// ---------------------------------------------------------------------------
// Tree traversal
// ---------------------------------------------------------------------------

// rootNode holds the top-level / directory. It is set during initialization
// and used by GetNode for path-based lookups.
var rootNode *treeNode

// GetNode traverses the configuration tree following the given path.
// The path is split on "/" and each segment is resolved via Children().
// Returns the node at the final path segment, or nil if not found.
func GetNode(path string) core.Node {
	path = strings.Trim(path, "/")
	if path == "" {
		return rootNode
	}

	segments := strings.Split(path, "/")
	current := core.Node(rootNode)

	for _, seg := range segments {
		dir, ok := current.(core.Directory)
		if !ok {
			return nil // cannot descend into a non-directory node
		}
		child, found := dir.Child(seg)
		if !found {
			return nil
		}
		current = child
	}

	return current
}

// ---------------------------------------------------------------------------
// Module loader
// ---------------------------------------------------------------------------

// registerModules builds the tree and registers all known modules.
func registerModules() (map[string]core.Node, error) {
	modules := make(map[string]core.Node)

	// 1. Create root /
	rootNode = newTreeNode("/", core.NodeTypeDirectory, "root")

	// 2. Create /ip
	ipDir := newTreeNode("/ip", core.NodeTypeDirectory, "IP")
	if err := rootNode.AddChild("ip", ipDir); err != nil {
		return nil, fmt.Errorf("failed to add /ip: %w", err)
	}

	// 3. Create a placeholder /interface checker with hardcoded interfaces.
	ifaceModule := interfaces.NewDefault()

	// 4. Build the IP address schema from the JSON definition embedded in code.
	//    In production this would be loaded from the module registry, but here
	//    we construct it manually for the simulator.
	schema := &config.ModuleSchema{
		Path:        "/ip/address",
		Type:        "list",
		Title:       "IP Addresses",
		Description: "Manages IPv4 addresses assigned to router interfaces.",
		Flags: []config.SchemaFlag{
			{Letter: "X", Name: "disabled", Description: "Entry is disabled."},
			{Letter: "I", Name: "invalid", Description: "Configuration is invalid."},
			{Letter: "D", Name: "dynamic", Description: "Entry created by a DHCP client."},
			{Letter: "S", Name: "slave", Description: "Address belongs to a slave interface."},
		},
		Schema: map[string]*config.SchemaProperty{
			"address": {
				Name: "address", Type: config.SchemaString, Required: true,
				Description: "IPv4 address with prefix length.",
			},
			"network": {
				Name: "network", Type: config.SchemaIPAddr,
				Description: "Network address derived from address and netmask.",
			},
			"broadcast": {
				Name: "broadcast", Type: config.SchemaIPAddr,
				Description: "Broadcast address derived from address and netmask.",
			},
			"interface": {
				Name: "interface", Type: config.SchemaInterface, Required: true,
				Description: "Interface on which the IP address is configured.",
			},
			"actual-interface": {
				Name: "actual-interface", Type: config.SchemaInterface, ReadOnly: true,
				Description: "Actual interface where the address is set up.",
			},
			"vrf": {
				Name: "vrf", Type: config.SchemaEnum, ReadOnly: true,
				Default: "main", Description: "VRF this address is associated with.",
			},
			"comment": {
				Name: "comment", Type: config.SchemaString, Default: "",
				Description: "User comment.",
			},
		},
		Actions: map[string]*config.SchemaAction{
			"add": {
				Name: "add", Parameters: []string{"address", "interface", "comment"},
				Validators:  []string{"duplicate_ip_per_interface", "valid_netmask", "interface_exists", "ip_not_in_reserved_range"},
				FlagsSet:    []string{"disabled"},
				Description: "Add a new IP address.",
			},
			"set": {
				Name: "set", Parameters: []string{"numbers", "address", "interface", "comment"},
				Validators:  []string{"entry_exists", "duplicate_ip_per_interface", "interface_exists"},
				Description: "Modify properties of existing IP address(es).",
			},
			"remove": {
				Name: "remove", Parameters: []string{"numbers"},
				Validators:  []string{"entry_exists", "not_dynamic"},
				Description: "Delete IP address(es).",
			},
			"disable": {
				Name: "disable", Parameters: []string{"numbers"},
				Validators:  []string{"entry_exists"},
				Description: "Disable IP address(es).",
			},
			"enable": {
				Name: "enable", Parameters: []string{"numbers"},
				Validators:  []string{"entry_exists"},
				Description: "Enable disabled IP address(es).",
			},
		},
		Defaults: map[string]interface{}{
			"comment":  "",
			"disabled": false,
			"vrf":      "main",
		},
		Constraints: map[string]string{
			"duplicate_ip_per_interface": "Cannot add the same IP address on the same interface.",
			"valid_netmask":              "Netmask must be between /0 and /32.",
			"interface_exists":           "Interface must exist in /interface.",
			"ip_not_in_reserved_range":   "Reserved IP ranges cannot be assigned.",
		},
	}

	ipAddrModule, err := ipAddr.New(schema, ifaceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPAddressModule: %w", err)
	}

	if err := ipDir.AddChild("address", ipAddrModule); err != nil {
		return nil, fmt.Errorf("failed to register /ip/address: %w", err)
	}

	modules["/ip/address"] = ipAddrModule
	return modules, nil
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

	// Verify traversal through GetNode
	testPaths := []string{
		"/",
		"/ip",
		"/ip/address",
		"/ip/address/nonexistent",
		"relative",
	}

	for _, p := range testPaths {
		node := GetNode(p)
		if node != nil {
			fmt.Printf("  %-30s → %s (%s)\n", p, node.Path(), node.Type())
		} else {
			fmt.Printf("  %-30s → <nil>\n", p)
		}
	}

	fmt.Println()
	fmt.Println("--- IP Address Module Verification ---")

	// Demonstrate IPAddressModule CRUD
	mod, ok := modules["/ip/address"].(*ipAddr.IPAddressModule)
	if !ok {
		fmt.Println("ERROR: /ip/address is not an IPAddressModule")
		return
	}

	// Add an entry
	entry, err := mod.Add(map[string]interface{}{
		"address":   "192.168.1.1/24",
		"interface": "ether1",
		"comment":   "LAN interface",
	})
	if err != nil {
		fmt.Printf("ERROR adding entry: %v\n", err)
		return
	}
	fmt.Printf("  Added:     ID=%s  address=%s  interface=%s\n",
		entry.ID(), entry.Properties()["address"], entry.Properties()["interface"])

	// Get the entry back
	got, ok := mod.Get(entry.ID())
	if !ok {
		fmt.Println("ERROR: entry not found via Get")
		return
	}
	fmt.Printf("  Get:       ID=%s  address=%s\n", got.ID(), got.Properties()["address"])

	// List entries
	entries := mod.List()
	fmt.Printf("  List:      %d entries\n", len(entries))

	// Try duplicate IP on same interface (should fail)
	_, err = mod.Add(map[string]interface{}{
		"address":   "192.168.1.1/24",
		"interface": "ether1",
	})
	if err != nil {
		fmt.Printf("  Duplicate: rejected (expected): %v\n", err)
	} else {
		fmt.Println("  Duplicate: allowed (unexpected)")
	}

	// Remove
	if err := mod.Remove(entry.ID()); err != nil {
		fmt.Printf("ERROR removing entry: %v\n", err)
	} else {
		fmt.Printf("  Removed:   ID=%s\n", entry.ID())
		entries = mod.List()
		fmt.Printf("  List:      %d entries after removal\n", len(entries))
	}

	fmt.Println()
	fmt.Println("=== Done ===")
}
