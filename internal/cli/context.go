package cli

import (
	"github.com/Devaste/MikroLab/internal/tree"
)

// Context holds the execution context for a specific device's tree.
// This allows the CLI executor to operate on per-device trees rather
// than the global tree.Root.
type Context struct {
	Root *tree.TreeNode
}

// NewContext creates a new CLI execution context for the given tree.
func NewContext(root *tree.TreeNode) *Context {
	return &Context{Root: root}
}

// GetRoot returns the root tree node for this context.
func (c *Context) GetRoot() *tree.TreeNode {
	return c.Root
}
