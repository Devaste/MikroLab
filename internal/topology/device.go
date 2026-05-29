// Package topology provides multi-device management and virtual network simulation
// for MikroLab. It allows creating multiple RouterOS devices, connecting them via
// virtual links, and forwarding packets between them.
package topology

import (
	"fmt"
	"sync"

	"github.com/Devaste/MikroLab/internal/cli"
	"github.com/Devaste/MikroLab/internal/tree"
)

// Device represents a single RouterOS simulator instance with its own
// independent configuration tree.
type Device struct {
	ID   string
	Name string
	Tree *tree.TreeNode // the configuration tree (root node)

	mu     sync.RWMutex
	topo   *Topology
	cliCtx *cli.Context
}

// NewDevice creates a new Device with the given ID and name.
// The tree is registered globally under Root for backward compatibility.
func NewDevice(id, name string, topo *Topology) (*Device, error) {
	if id == "" {
		return nil, fmt.Errorf("device ID is required")
	}
	if name == "" {
		name = id
	}

	// Create a fresh configuration tree for this device
	deviceTree := tree.NewTree()

	// Create a CLI context for this device
	cliCtx := cli.NewContext(deviceTree)

	return &Device{
		ID:     id,
		Name:   name,
		Tree:   deviceTree,
		topo:   topo,
		cliCtx: cliCtx,
	}, nil
}

// ExecuteCommand executes a RouterOS command on this device's configuration tree.
func (d *Device) ExecuteCommand(line string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Parse the command
	parsed, err := cli.Parse(line)
	if err != nil {
		return "", err
	}

	// Execute against this device's tree
	output, err := cli.ExecuteOnTree(parsed, d.cliCtx)
	if err != nil {
		return "", err
	}

	return output, nil
}

// ExecuteAPI executes an API-style command on this device.
func (d *Device) ExecuteAPI(cmd string, params map[string]interface{}) (interface{}, error) {
	return cli.ExecuteAPIOnTree(cmd, params, d.cliCtx)
}
