// Package tree provides the concrete tree node implementation for the MikroLab
// configuration tree. It implements the core.Node and core.Directory interfaces
// and provides path-based tree traversal via GetNode.
package tree

import (
	"fmt"
	"strings"

	"github.com/Devaste/MikroLab/internal/core"
)

// TreeNode is a concrete directory node that implements core.Directory.
// It stores child nodes in a map and satisfies all interfaces needed to
// build the configuration tree.
type TreeNode struct {
	path     string
	nodeType core.NodeType
	title    string
	children map[string]core.Node
}

// NewTreeNode creates a new TreeNode with the given path, type, and title.
func NewTreeNode(path string, nodeType core.NodeType, title string) *TreeNode {
	return &TreeNode{
		path:     path,
		nodeType: nodeType,
		title:    title,
		children: make(map[string]core.Node),
	}
}

// NewTree creates a new independent tree with a root node "/".
func NewTree() *TreeNode {
	return NewTreeNode("/", core.NodeTypeDirectory, "root")
}

// core.Node interface
func (n *TreeNode) Path() string        { return n.path }
func (n *TreeNode) Type() core.NodeType { return n.nodeType }
func (n *TreeNode) Title() string       { return n.title }

// core.Directory interface
func (n *TreeNode) Children() map[string]core.Node { return n.children }
func (n *TreeNode) Child(name string) (core.Node, bool) {
	child, ok := n.children[name]
	return child, ok
}
func (n *TreeNode) AddChild(name string, child core.Node) error {
	if _, exists := n.children[name]; exists {
		return fmt.Errorf("child %q already exists under %q", name, n.path)
	}
	n.children[name] = child
	return nil
}
func (n *TreeNode) RemoveChild(name string) error {
	if _, exists := n.children[name]; !exists {
		return fmt.Errorf("child %q not found under %q", name, n.path)
	}
	delete(n.children, name)
	return nil
}

// Compile-time checks
var _ core.Node = (*TreeNode)(nil)
var _ core.Directory = (*TreeNode)(nil)

// Root holds the top-level / directory. It must be set during initialization
// before any calls to GetNode.
var Root *TreeNode

// GetNode traverses the configuration tree following the given path.
// The path is split on "/" and each segment is resolved via Children().
// Returns the node at the final path segment, or nil if not found.
// Uses the global Root by default. For per-device trees, use GetNodeOnTree.
func GetNode(path string) core.Node {
	if Root == nil {
		return nil
	}
	return GetNodeOnTree(Root, path)
}

// GetNodeOnTree traverses the given tree root following the given path.
func GetNodeOnTree(root *TreeNode, path string) core.Node {
	if root == nil {
		return nil
	}

	path = strings.Trim(path, "/")
	if path == "" {
		return root
	}

	segments := strings.Split(path, "/")
	current := core.Node(root)

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
