// Package core defines the foundational interfaces for the MikroLab
// configuration tree, reflecting RouterOS 7's structure.
package core

// NodeType represents the type of a configuration tree node.
type NodeType string

const (
	// NodeTypeDirectory is a node that contains child nodes (sub-directories or lists).
	NodeTypeDirectory NodeType = "directory"

	// NodeTypeList is a node that holds an ordered collection of entries.
	NodeTypeList NodeType = "list"

	// NodeTypeSingleton is a node holding a single entry (e.g., /system identity).
	NodeTypeSingleton NodeType = "singleton"
)

// Node is the base interface for every path in the configuration tree.
// Each node has a path, type, and display title.
type Node interface {
	// Path returns the full absolute path of this node (e.g., "/ip/address").
	Path() string

	// Type returns the node type.
	Type() NodeType

	// Title returns the human-readable display name.
	Title() string
}

// Directory is a node that can contain child nodes.
// In RouterOS 7, directories act as namespaces for sub-directories or setting lists.
type Directory interface {
	Node

	// Children returns a map of child node names to their Node interface values.
	// The map key is the last path segment (e.g., "address" for /ip/address).
	Children() map[string]Node

	// AddChild inserts a child node at the given name. Returns an error if
	// the name is already taken or the directory cannot accept children.
	AddChild(name string, child Node) error

	// RemoveChild deletes the child with the given name. Returns an error if
	// the child does not exist or cannot be removed.
	RemoveChild(name string) error

	// Child returns the child node with the given name, or false if not found.
	Child(name string) (Node, bool)
}
