package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/tree"
)

// Execute parses and runs a single command against the configuration tree.
//
// It uses tree.GetNode to resolve the path, checks that the node implements
// core.SettingsDirectory, and performs the requested action.
func Execute(cmd ParsedCommand) (string, error) {
	// 1. Resolve the path
	node := tree.GetNode(cmd.Path)
	if node == nil {
		return "", fmt.Errorf("failure: path %q not found", cmd.Path)
	}

	// 2. Assert that the node is a SettingsDirectory
	mod, ok := node.(core.SettingsDirectory)
	if !ok {
		return "", fmt.Errorf("failure: %q is not a settings directory", cmd.Path)
	}

	// 3. Perform the action
	switch cmd.Action {
	case "print":
		return doPrint(mod), nil

	case "add":
		return doAdd(mod, cmd.Params)

	case "remove":
		return doRemove(mod, cmd.Params)

	case "set":
		return doSet(mod, cmd.Params)

	case "enable":
		// enable sets disabled=false
		params := copyParams(cmd.Params)
		params["disabled"] = "false" // this will be overwritten if user also passed disabled=...
		return doSet(mod, params)

	case "disable":
		// disable sets disabled=true
		params := copyParams(cmd.Params)
		params["disabled"] = "true"
		return doSet(mod, params)

	default:
		return "", fmt.Errorf("failure: unknown action %q", cmd.Action)
	}
}

// doPrint lists all entries and formats them as a table.
func doPrint(mod core.SettingsDirectory) string {
	entries := mod.List()
	return FormatTable(entries)
}

// doAdd creates a new entry from the given parameters.
func doAdd(mod core.SettingsDirectory, params map[string]string) (string, error) {
	if len(params) == 0 {
		return "", fmt.Errorf("failure: no parameters provided for add")
	}

	props := stringMapToInterfaceMap(params)
	entry, err := mod.Add(props)
	if err != nil {
		return "", fmt.Errorf("failure: %v", err)
	}

	return fmt.Sprintf("Added entry with ID = %s\n", entry.ID()), nil
}

// doRemove removes entries by numeric index (numbers parameter).
func doRemove(mod core.SettingsDirectory, params map[string]string) (string, error) {
	numbersRaw, ok := params["numbers"]
	if !ok || numbersRaw == "" {
		return "", fmt.Errorf("failure: 'numbers' parameter is required for remove")
	}

	ids, err := parseNumbersToIDs(mod, numbersRaw)
	if err != nil {
		return "", fmt.Errorf("failure: %v", err)
	}

	count := 0
	for _, id := range ids {
		if err := mod.Remove(id); err != nil {
			// If some entries fail, report partial failure
			return "", fmt.Errorf("failure: failed to remove entry %q: %v", id, err)
		}
		count++
	}

	return fmt.Sprintf("Removed %d entries\n", count), nil
}

// doSet updates entries by numeric index (numbers parameter) with the given params.
func doSet(mod core.SettingsDirectory, params map[string]string) (string, error) {
	numbersRaw, ok := params["numbers"]
	if !ok || numbersRaw == "" {
		return "", fmt.Errorf("failure: 'numbers' parameter is required for set")
	}

	ids, err := parseNumbersToIDs(mod, numbersRaw)
	if err != nil {
		return "", fmt.Errorf("failure: %v", err)
	}

	// Build the property map, excluding "numbers"
	props := make(map[string]string)
	for k, v := range params {
		if k == "numbers" {
			continue
		}
		props[k] = v
	}

	if len(props) == 0 {
		return "", fmt.Errorf("failure: no properties provided for set")
	}

	interfaceProps := stringMapToInterfaceMap(props)

	count := 0
	for _, id := range ids {
		if err := mod.Set(id, interfaceProps); err != nil {
			return "", fmt.Errorf("failure: failed to set entry %q: %v", id, err)
		}
		count++
	}

	return fmt.Sprintf("Set on %d entries\n", count), nil
}

// parseNumbersToIDs converts a numbers specification (e.g., "0,2-4") into
// a list of actual entry IDs by looking up indices in the module's List().
func parseNumbersToIDs(mod core.SettingsDirectory, numbersRaw string) ([]string, error) {
	indices, err := ParseNumbers(numbersRaw)
	if err != nil {
		return nil, err
	}

	if indices == nil {
		// "*" was specified – return all entry IDs
		entries := mod.List()
		ids := make([]string, len(entries))
		for i, e := range entries {
			ids[i] = e.ID()
		}
		return ids, nil
	}

	entries := mod.List()
	entriesByIndex := make(map[int]core.Entry)
	for _, e := range entries {
		entriesByIndex[e.Index()] = e
	}

	var ids []string
	for _, idxStr := range indices {
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, fmt.Errorf("invalid index %q", idxStr)
		}

		e, found := entriesByIndex[idx]
		if !found {
			return nil, fmt.Errorf("entry at index %d not found", idx)
		}
		ids = append(ids, e.ID())
	}

	return ids, nil
}

// stringMapToInterfaceMap converts map[string]string to map[string]interface{}.
func stringMapToInterfaceMap(src map[string]string) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyParams creates a shallow copy of a string map.
func copyParams(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ParseEnabled is a helper to parse a "disabled" parameter value from RouterOS.
// RouterOS stores boolean properties as strings: "true", "false", "yes", "no", "1", "0".
func ParseEnabled(val string) (bool, error) {
	switch strings.ToLower(val) {
	case "true", "yes", "1":
		return true, nil
	case "false", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", val)
	}
}
