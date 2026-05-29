// Package tree provides the concrete tree node implementation for the MikroLab
// configuration tree. It implements the core.Node and core.Directory interfaces
// and provides path-based tree traversal via GetNode.
package tree

import (
	"fmt"
	"strings"

	"github.com/Devaste/MikroLab/internal/core"
)

// treeNode is a concrete directory node that implements core.Directory.
// It stores child nodes in a map and satisfies all interfaces needed to
// build the configuration tree.
type treeNode struct {
	path     string
	nodeType core.NodeType
	title    string
	children map[string]core.Node
}

// NewTreeNode creates a new treeNode with the given path, type, and title.
func NewTreeNode(path string, nodeType core.NodeType, title string) *treeNode {
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

// Compile-time checks
var _ core.Node = (*treeNode)(nil)
var _ core.Directory = (*treeNode)(nil)

// Root holds the top-level / directory. It must be set during initialization
// before any calls to GetNode.
var Root *treeNode

// GetNode traverses the configuration tree following the given path.
// The path is split on "/" and each segment is resolved via Children().
// Returns the node at the final path segment, or nil if not found.
func GetNode(path string) core.Node {
	if Root == nil {
		return nil
	}

	path = strings.Trim(path, "/")
	if path == "" {
		return Root
	}

	segments := strings.Split(path, "/")
	current := core.Node(Root)

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
