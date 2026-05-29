package cli

import (
	"fmt"
	"strings"
)

// ExecuteAPI executes a RouterOS command from structured API parameters.
//
// It reconstructs a CLI string from the command path and params, then
// delegates to the existing Parse/Execute pipeline. This keeps the existing
// CLI parser unchanged while providing a clean API entry point.
//
// For commands without params (e.g., "/ip/address/print"), the command
// string is used as-is.
//
// For commands with params (e.g., "/ip/address/add", {"address": "..."}),
// param keys are reconstructed in "key=value" format.
func ExecuteAPI(cmd string, params map[string]interface{}) (interface{}, error) {
	// Reconstruct the CLI string
	cliStr := reconstructCommand(cmd, params)

	// Parse the CLI string
	parsed, err := Parse(cliStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	// Execute through the existing pipeline
	output, err := Execute(parsed)
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
