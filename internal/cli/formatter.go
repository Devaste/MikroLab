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

// knownColumns defines the columns to display for /ip/address and similar
// list modules. In a future version, this could be derived from the schema.
var knownColumns = []propertyColumn{
	{Name: "address", Title: "ADDRESS", Width: 18},
	{Name: "interface", Title: "INTERFACE", Width: 12},
	{Name: "comment", Title: "COMMENT", Width: 20},
	{Name: "disabled", Title: "DISABLED", Width: 2}, // shown as flag
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
	b.WriteString("Flags: X - disabled, I - invalid, D - dynamic, S - slave\n")

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

	// Calculate dynamic column widths based on actual data
	colWidths := make([]int, len(knownColumns))
	for i, col := range knownColumns {
		colWidths[i] = col.Width
		if col.Name == "disabled" {
			continue // flag column, fixed width
		}
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
	for i, col := range knownColumns {
		if col.Name == "disabled" {
			continue // flags are shown in the # column area
		}
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
		for i, col := range knownColumns {
			if col.Name == "disabled" {
				continue
			}
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

// formatFlags returns a string of flag characters for an entry.
// Spaces are used for flags that are not set.
//
// RouterOS convention:
//
//	X - disabled
//	I - invalid
//	D - dynamic
//	S - slave
func formatFlags(entry core.Entry) string {
	flags := entry.Flags()
	if flags == nil {
		return " "
	}

	var b strings.Builder
	if flags["disabled"] {
		b.WriteByte('X')
	} else {
		b.WriteByte(' ')
	}
	if flags["invalid"] {
		b.WriteByte('I')
	} else {
		b.WriteByte(' ')
	}
	if flags["dynamic"] {
		b.WriteByte('D')
	} else {
		b.WriteByte(' ')
	}
	if flags["slave"] {
		b.WriteByte('S')
	} else {
		b.WriteByte(' ')
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
