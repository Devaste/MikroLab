package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Devaste/MikroLab/internal/config"
	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/modules/interfaces"
	ipAddr "github.com/Devaste/MikroLab/internal/modules/ip/address"
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

	// 4. Load the IP address schema from the JSON file.
	schemaData, err := os.ReadFile("internal/modules/ip/address/schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read IP address schema: %w", err)
	}
	schema := &config.ModuleSchema{}
	if err := json.Unmarshal(schemaData, schema); err != nil {
		return nil, fmt.Errorf("failed to parse IP address schema: %w", err)
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
