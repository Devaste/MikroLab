package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Devaste/MikroLab/internal/core"
)

// propertyColumn defines a column in the print table.
type propertyColumn struct {
	Name  string // property key in the entry
	Title string // column header
	Width int    // minimum width for alignment
}

// columnsToExclude lists property names that should never appear as
// table columns (they are either represented as flags or are internal).
var columnsToExclude = map[string]bool{
	"disabled":   true,
	"running":    true,
	"dynamic":    true,
	"inactive":   true,
	"actual-mtu": true,
	"active":     true,
}

// columnDisplayNames maps property names to human-readable column headers.
var columnDisplayNames = map[string]string{
	"address":       "ADDRESS",
	"interface":     "INTERFACE",
	"name":          "NAME",
	"default-name":  "DEFAULT-NAME",
	"type":          "TYPE",
	"mtu":           "MTU",
	"l2mtu":         "L2MTU",
	"mac-address":   "MAC-ADDRESS",
	"comment":       "COMMENT",
	"dst-address":   "DST-ADDRESS",
	"gateway":       "GATEWAY",
	"distance":      "DISTANCE",
	"pref-src":      "PREF-SRC",
	"routing-table": "ROUTING-TABLE",
	"blackhole":     "BLACKHOLE",
	"check-gateway": "CHECK-GATEWAY",
	"immediate-gw":  "IMMEDIATE-GW",
	"local-address": "LOCAL-ADDRESS",
}

// determineColumns derives the column list from the first available entry's
// properties, excluding internal/flag properties and ordering known columns
// before unknown ones.
func determineColumns(entries []core.Entry) []propertyColumn {
	if len(entries) == 0 {
		// No entries — return a sensible default for the module type
		return []propertyColumn{
			{Name: "name", Title: "NAME", Width: 10},
			{Name: "type", Title: "TYPE", Width: 6},
			{Name: "comment", Title: "COMMENT", Width: 10},
		}
	}

	// Collect all property names from the first entry
	first := entries[0]
	allProps := first.Properties()

	// Build the column list: known columns first (in a preferred order),
	// then any unknown columns alphabetically.
	preferredOrder := []string{
		"address", "interface", "name", "default-name", "type",
		"mtu", "l2mtu", "mac-address", "comment",
		"dst-address", "gateway", "distance",
	}

	var cols []propertyColumn
	seen := make(map[string]bool)

	// Add known columns in preferred order
	for _, name := range preferredOrder {
		if _, exists := allProps[name]; !exists {
			continue
		}
		if columnsToExclude[name] {
			continue
		}
		title := columnDisplayNames[name]
		if title == "" {
			title = strings.ToUpper(name)
		}
		width := len(title)
		// Find the longest value
		for _, entry := range entries {
			if v, ok := entry.Property(name); ok && v != nil {
				s := fmt.Sprintf("%v", v)
				if len(s) > width {
					width = len(s)
				}
			}
		}
		cols = append(cols, propertyColumn{Name: name, Title: title, Width: width})
		seen[name] = true
	}

	// Add any remaining properties not in preferred order
	var remaining []string
	for name := range allProps {
		if seen[name] || columnsToExclude[name] {
			continue
		}
		remaining = append(remaining, name)
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		title := columnDisplayNames[name]
		if title == "" {
			title = strings.ToUpper(name)
		}
		width := len(title)
		for _, entry := range entries {
			if v, ok := entry.Property(name); ok && v != nil {
				s := fmt.Sprintf("%v", v)
				if len(s) > width {
					width = len(s)
				}
			}
		}
		cols = append(cols, propertyColumn{Name: name, Title: title, Width: width})
	}

	return cols
}

// formatFlagsLegend builds the flags legend line based on the first entry's flags.
func formatFlagsLegend(entries []core.Entry) string {
	if len(entries) == 0 {
		return "Flags: X - disabled, I - invalid, D - dynamic, S - slave\n"
	}

	flags := entries[0].Flags()
	if flags == nil {
		return "Flags: X - disabled, I - invalid, D - dynamic, S - slave\n"
	}

	var parts []string

	// Standard flags always shown
	parts = append(parts, "X - disabled")
	parts = append(parts, "I - invalid")
	parts = append(parts, "D - dynamic")

	// Check for module-specific flags on the first entry
	if _, hasH := flags["dhcp"]; hasH {
		parts = append(parts, "H - dhcp")
	}
	if _, hasP := flags["published"]; hasP {
		parts = append(parts, "P - published")
	}
	if _, hasC := flags["complete"]; hasC {
		parts = append(parts, "C - complete")
	}
	if _, hasS := flags["slave"]; hasS {
		parts = append(parts, "S - slave")
	}
	if _, hasA := flags["active"]; hasA {
		parts = append(parts, "A - active")
	}
	if _, hasConnect := flags["connect"]; hasConnect {
		parts = append(parts, "c - connect")
	}
	if _, hasStatic := flags["static"]; hasStatic {
		parts = append(parts, "s - static")
	}
	if _, hasRunning := flags["running"]; hasRunning {
		parts = append(parts, "R - running")
	}
	if _, hasInactive := flags["inactive"]; hasInactive {
		parts = append(parts, "I - inactive")
	}

	return "Flags: " + strings.Join(parts, ", ") + "\n"
}

// FormatTable formats a list of entries into a RouterOS-style aligned table.
//
// Example output:
//
//	Flags: X - disabled, I - invalid, D - dynamic, S - slave
//	 #  ADDRESS          INTERFACE  COMMENT
//	 0  192.168.1.1/24   ether1     LAN
//	 1  10.0.0.1/24      ether2
func FormatTable(entries []core.Entry) string {
	var b strings.Builder

	// Print flags legend
	b.WriteString(formatFlagsLegend(entries))

	// Determine the width for the "#" column based on the number of entries
	numWidth := 1
	if len(entries) > 0 {
		// width of the largest index number
		largest := len(entries) - 1
		for largest >= 10 {
			numWidth++
			largest /= 10
		}
	}
	if numWidth < 1 {
		numWidth = 1
	}

	// Determine columns dynamically from the first entry
	columns := determineColumns(entries)

	// Calculate dynamic column widths based on actual data
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = col.Width
		// Find the longest value
		for _, entry := range entries {
			if v, ok := entry.Property(col.Name); ok && v != nil {
				s := fmt.Sprintf("%v", v)
				if len(s) > colWidths[i] {
					colWidths[i] = len(s)
				}
			}
		}
	}

	// Print header row
	b.WriteString(" ")
	for i := 0; i < numWidth; i++ {
		b.WriteByte(' ')
	}
	b.WriteString("  ")
	for i, col := range columns {
		b.WriteString(fmt.Sprintf("%-*s  ", colWidths[i], col.Title))
	}
	b.WriteString("\n")

	// Print each entry
	for _, entry := range entries {
		// Flags
		flags := formatFlags(entry)

		// Index with flags — use entry.Index() not loop index
		fmt.Fprintf(&b, " %s%-*d", flags, numWidth, entry.Index())

		// Properties
		for i, col := range columns {
			val := ""
			if v, ok := entry.Property(col.Name); ok && v != nil {
				val = fmt.Sprintf("%v", v)
			}
			fmt.Fprintf(&b, "  %-*s", colWidths[i], val)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// flagLetter maps flag names to their RouterOS display letters.
var flagLetters = map[string]byte{
	"disabled":  'X',
	"dynamic":   'D',
	"invalid":   'I',
	"slave":     'S',
	"active":    'A',
	"connect":   'c',
	"static":    's',
	"published": 'P',
	"complete":  'C',
}

// flagRenderOrder defines the order in which flags are rendered.
var flagRenderOrder = []string{
	"active", "dynamic", "disabled", "connect", "static", "invalid", "slave",
}

// formatFlags returns a string of flag characters for an entry.
// Spaces are used for flags that are not set.
//
// RouterOS convention:
//
//	X - disabled
//	I - invalid
//	D - dynamic
//	S - slave
//	A - active
//	c - connect
//	s - static
//	R - running
//	P - published
//	C - complete
func formatFlags(entry core.Entry) string {
	flags := entry.Flags()
	if flags == nil {
		return " "
	}

	var b strings.Builder

	// Use the standard order for common flags
	for _, name := range flagRenderOrder {
		if letter, ok := flagLetters[name]; ok {
			if flags[name] {
				b.WriteByte(letter)
			} else {
				b.WriteByte(' ')
			}
		}
	}

	// Append any module-specific flags not in the standard order
	moduleFlags := []string{"published", "complete", "dhcp"}
	for _, name := range moduleFlags {
		if val, exists := flags[name]; exists {
			if val {
				if letter, ok := flagLetters[name]; ok {
					b.WriteByte(letter)
				} else {
					// Fallback: just show the first letter
					if len(name) > 0 {
						b.WriteByte(name[0] - 32) // uppercase first char
					}
				}
			}
		}
	}

	return b.String()
}

// sortedPropertyKeys returns the property keys of an entry sorted alphabetically.
// Used as a fallback when knownColumns don't match.
func sortedPropertyKeys(entry core.Entry) []string {
	props := entry.Properties()
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// propertyDisplayValue returns the string representation of a property value
// for display in the table.
func propertyDisplayValue(entry core.Entry, name string) string {
	v, ok := entry.Property(name)
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
