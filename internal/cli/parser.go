// Package cli implements a RouterOS-style CLI interpreter for the MikroLab
// configuration tree. It provides command parsing, execution, and output
// formatting.
package cli

import (
	"fmt"
	"strings"
)

// ParsedCommand holds the structured representation of a parsed CLI command.
type ParsedCommand struct {
	Path   string            // e.g., "/ip/address"
	Action string            // "print", "add", "remove", "set", "enable", "disable"
	Params map[string]string // e.g., {"address": "192.168.1.1/24", "interface": "ether1"}
}

// knownActions is the set of action verbs the parser recognises.
var knownActions = map[string]bool{
	"print":   true,
	"add":     true,
	"remove":  true,
	"set":     true,
	"enable":  true,
	"disable": true,
}

// Parse parses a RouterOS-style command string into a ParsedCommand.
//
// Examples:
//
//	/ip address print
//	/ip address add address=192.168.1.1/24 interface=ether1 comment=LAN
//	/ip address remove numbers=0
//	/ip address set numbers=0 address=10.0.0.1/24
//
// Rules:
//   - Path is everything before the action verb.
//   - Action is the verb (print/add/remove/set/enable/disable).
//   - Everything after action is space-separated key=value pairs.
//   - Extra spaces are ignored.
func Parse(input string) (ParsedCommand, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return ParsedCommand{}, fmt.Errorf("empty command")
	}

	// Split on whitespace, handling multiple spaces
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return ParsedCommand{}, fmt.Errorf("empty command")
	}

	// Find the action verb. Scan tokens from right to left so that we
	// correctly identify the last token that is a known action.
	actionIdx := -1
	for i, tok := range tokens {
		if knownActions[tok] {
			actionIdx = i
			break
		}
	}

	if actionIdx == -1 {
		return ParsedCommand{}, fmt.Errorf("no action verb found in command %q", input)
	}

	// Path is everything before the action verb
	pathTokens := tokens[:actionIdx]
	if len(pathTokens) == 0 {
		return ParsedCommand{}, fmt.Errorf("missing path before action %q", tokens[actionIdx])
	}

	// Normalise path: ensure leading "/" and join segments with "/"
	path := strings.Join(pathTokens, "/")
	path = normalizePath(path)

	action := tokens[actionIdx]

	// Params are the remaining tokens after the action
	params := make(map[string]string)
	for _, tok := range tokens[actionIdx+1:] {
		// Split on first "=" only
		if idx := strings.IndexByte(tok, '='); idx >= 0 {
			key := tok[:idx]
			value := tok[idx+1:]
			if key != "" {
				params[key] = value
			}
		} else {
			// Token without "=" – treat as a boolean flag (key present, no value)
			// RouterOS-style: some commands accept bare flags like "disabled"
			params[tok] = ""
		}
	}

	return ParsedCommand{
		Path:   path,
		Action: action,
		Params: params,
	}, nil
}

// normalizePath ensures a leading "/" and cleans up path formatting.
func normalizePath(path string) string {
	// Ensure leading slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Remove trailing slash
	path = strings.TrimSuffix(path, "/")

	// Collapse multiple slashes
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}
