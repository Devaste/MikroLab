package cli

import (
	"fmt"
	"strings"

	"github.com/Devaste/MikroLab/internal/tree"
)

// ExecuteAPI executes a RouterOS command from structured API parameters.
//
// It reconstructs a CLI string from the command path and params, then
// delegates to the existing Parse/Execute pipeline.
func ExecuteAPI(cmd string, params map[string]interface{}) (interface{}, error) {
	return ExecuteAPIOnTree(cmd, params, &Context{Root: tree.Root})
}

// ExecuteAPIOnTree executes a RouterOS command on a specific device tree.
func ExecuteAPIOnTree(cmd string, params map[string]interface{}, ctx *Context) (interface{}, error) {
	// Reconstruct the CLI string
	cliStr := reconstructCommand(cmd, params)

	// Parse the CLI string
	parsed, err := Parse(cliStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	// Execute through the existing pipeline on the given tree
	output, err := ExecuteOnTree(parsed, ctx)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// reconstructCommand builds a CLI string from a command path and params.
//
// Examples:
//
//	"/ip/address/print", nil               -> "/ip/address/print"
//	"/ping", {"address":"8.8.8.8","count":"2"} -> "/ping 8.8.8.8 count=2"
func reconstructCommand(cmd string, params map[string]interface{}) string {
	if len(params) == 0 {
		return cmd
	}

	var b strings.Builder
	b.WriteString(cmd)

	for k, v := range params {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(fmt.Sprintf("%v", v))
	}

	return b.String()
}
