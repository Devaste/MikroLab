package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseNumbers parses a RouterOS-style numbers specification and returns
// a list of string IDs for each individual entry.
//
// Supported formats:
//   - "0"           → ["0"]
//   - "0,2-4"       → ["0", "2", "3", "4"]
//   - "0,2"         → ["0", "2"]
//   - "0-2"         → ["0", "1", "2"]
//   - "*"           → nil (reserved for "all"; not fully implemented here)
func ParseNumbers(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty numbers specification")
	}

	// Handle "*" (all) – for now return nil meaning "caller must handle"
	if raw == "*" {
		return nil, nil
	}

	var ids []string
	// Split on comma
	parts := strings.Split(raw, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this part is a range (contains "-")
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range %q", part)
			}

			startStr := strings.TrimSpace(rangeParts[0])
			endStr := strings.TrimSpace(rangeParts[1])

			if startStr == "" || endStr == "" {
				return nil, fmt.Errorf("invalid range %q: empty start or end", part)
			}

			start, err := strconv.Atoi(startStr)
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q: %w", startStr, err)
			}

			end, err := strconv.Atoi(endStr)
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q: %w", endStr, err)
			}

			if start < 0 || end < 0 {
				return nil, fmt.Errorf("negative numbers not allowed in range %q", part)
			}

			if start > end {
				// RouterOS does not swap; this is an error
				return nil, fmt.Errorf("invalid range %q: start > end", part)
			}

			for i := start; i <= end; i++ {
				ids = append(ids, strconv.Itoa(i))
			}
		} else {
			// Single number
			id, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid number %q: %w", part, err)
			}
			if id < 0 {
				return nil, fmt.Errorf("negative number not allowed: %q", part)
			}
			ids = append(ids, strconv.Itoa(id))
		}
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid numbers in %q", raw)
	}

	return ids, nil
}
