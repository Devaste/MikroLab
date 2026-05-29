package config

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// Max string length constants for input validation.
const (
	MaxPropertyStringLen  = 4096 // Max length for string property values
	MaxCommentLen         = 1024 // Max length for comment fields
	MaxPathSegmentLen     = 128  // Max length for a single path segment
	MaxSchemaTitleLen     = 256  // Max length for schema title
	MaxDescriptionLen     = 4096 // Max length for schema descriptions
	MaxNumbersInOperation = 1000 // Max number of entry references in a single operation
)

// SanitizeString trims whitespace and strips control characters from the input,
// enforcing a maximum length. Returns the sanitized string.
func SanitizeString(s string, maxLen int) string {
	// Trim leading/trailing whitespace
	s = strings.TrimSpace(s)

	// Strip control characters (keep tabs and newlines? We'll strip everything below 0x20)
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7F { // printable ASCII and Unicode; skip DEL
			b.WriteRune(r)
		}
	}
	s = b.String()

	// Enforce max length
	if len(s) > maxLen {
		// Use rune-based truncation to avoid splitting multi-byte characters
		runes := []rune(s)
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}
		return string(runes)
	}

	return s
}

// SanitizeComment sanitizes a comment field — allows printable chars only.
func SanitizeComment(s string, maxLen int) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(r)
		}
	}
	s = b.String()
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		runes := []rune(s)
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}
		s = string(runes)
	}
	return s
}

// SanitizeCLIInput sanitizes string values that originated from CLI or .rsc scripts.
// In addition to base sanitization, it strips characters that could be interpreted
// as shell metacharacters or CLI injection vectors.
func SanitizeCLIInput(s string, maxLen int) string {
	s = SanitizeString(s, maxLen)

	// Additional CLI protection for strings that may be used in contexts
	// where they could be re-interpreted:
	// Strip backticks, $, {, }, (, ), ;, |, &, <, >, \, ', "
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '`', '$', '{', '}', '(', ')', ';', '|', '&', '<', '>', '\\', '\'', '"':
			continue // strip shell metacharacters
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CoercePropertyValue attempts to coerce a raw interface{} value to match
// the expected SchemaPropertyType. Returns the coerced value or an error
// if coercion is impossible.
func CoercePropertyValue(value interface{}, propType SchemaPropertyType) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch propType {
	case SchemaString:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}
		return SanitizeString(s, MaxPropertyStringLen), nil

	case SchemaInteger:
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			i, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("invalid integer %q: %w", v, err)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("expected integer, got %T", value)
		}

	case SchemaBoolean:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "true", "yes", "1":
				return true, nil
			case "false", "no", "0":
				return false, nil
			default:
				return nil, fmt.Errorf("invalid boolean string %q", v)
			}
		case float64:
			return v != 0, nil
		default:
			return nil, fmt.Errorf("expected boolean, got %T", value)
		}

	case SchemaIPAddr:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected IP address string, got %T", value)
		}
		s = SanitizeString(s, 64)
		if net.ParseIP(s) == nil {
			return nil, fmt.Errorf("invalid IP address %q", s)
		}
		return s, nil

	case SchemaMACAddr:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected MAC address string, got %T", value)
		}
		s = SanitizeString(s, 32)
		valid := validMACFormat(s)
		if !valid {
			return nil, fmt.Errorf("invalid MAC address format %q", s)
		}
		return s, nil

	case SchemaIPPrefix:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected IP prefix string, got %T", value)
		}
		s = SanitizeString(s, 64)
		_, _, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR notation %q: %w", s, err)
		}
		return s, nil

	case SchemaEnum, SchemaInterface:
		// For enum and interface types, accept string only but
		// do not validate against a closed set here (validated elsewhere)
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected string, got %T", value)
		}
		return SanitizeString(s, MaxPropertyStringLen), nil

	case SchemaComposite, SchemaCompositeIP:
		// Composite types (e.g., "192.168.1.1/24") are stored as strings
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected composite string, got %T", value)
		}
		return SanitizeString(s, MaxPropertyStringLen), nil

	default:
		// Unknown type — accept as-is but sanitize if string
		if s, ok := value.(string); ok {
			return SanitizeString(s, MaxPropertyStringLen), nil
		}
		return value, nil
	}
}

// validMACFormat checks if the string matches a known MAC address format.
func validMACFormat(s string) bool {
	patterns := []string{
		`^([0-9A-Fa-f]{2}[:]){5}[0-9A-Fa-f]{2}$`,
		`^([0-9A-Fa-f]{2}[-]){5}[0-9A-Fa-f]{2}$`,
		`^([0-9A-Fa-f]{4}[.]){2}[0-9A-Fa-f]{4}$`,
	}
	for _, p := range patterns {
		matched, _ := regexp.MatchString(p, s)
		if matched {
			return true
		}
	}
	return false
}

// ValidateEntryID validates that an entry ID is a valid UUID.
// Empty string is allowed (e.g., for "add" operations before ID assignment).
func ValidateEntryID(id string) error {
	if id == "" {
		return nil // empty is valid (not yet assigned)
	}
	_, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid entry ID %q: %w", id, err)
	}
	return nil
}

// ValidateNumbers validates the numbers field of an operation.
// Each entry should be a numeric reference (e.g., "0", "1", "2", "0-5").
func ValidateNumbers(numbers []string) error {
	if len(numbers) == 0 {
		return nil
	}
	if len(numbers) > MaxNumbersInOperation {
		return fmt.Errorf("too many numbers: %d (max %d)", len(numbers), MaxNumbersInOperation)
	}

	for _, n := range numbers {
		n = strings.TrimSpace(n)
		if n == "" {
			return fmt.Errorf("empty number reference")
		}

		// Check for ranges like "0-5"
		if strings.Contains(n, "-") {
			parts := strings.Split(n, "-")
			if len(parts) != 2 {
				return fmt.Errorf("invalid number range %q", n)
			}
			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return fmt.Errorf("invalid number in range %q: %w", n, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return fmt.Errorf("invalid number in range %q: %w", n, err)
			}
			if start < 0 || end < 0 || start > end {
				return fmt.Errorf("invalid number range %q", n)
			}
		} else {
			idx, err := strconv.Atoi(n)
			if err != nil {
				return fmt.Errorf("invalid number reference %q: %w", n, err)
			}
			if idx < 0 {
				return fmt.Errorf("negative number reference %q", n)
			}
		}
	}
	return nil
}

// ValidateWhere validates the where clause of an operation.
// Fields should reference valid schema property names and have valid values.
func ValidateWhere(where map[string]interface{}, schema *ModuleSchema) error {
	for k, v := range where {
		if k == "" {
			return fmt.Errorf("empty field name in where clause")
		}

		// Check field references a real schema property (if schema is known)
		if schema != nil {
			prop, exists := schema.GetProperty(k)
			if !exists {
				return fmt.Errorf("where clause references non-existent property %q", k)
			}
			// Try to coerce the value type
			_, err := CoercePropertyValue(v, prop.Type)
			if err != nil {
				return fmt.Errorf("where clause value for %q: %w", k, err)
			}
		} else {
			// Without schema context, just sanitize strings
			if s, ok := v.(string); ok {
				where[k] = SanitizeString(s, MaxPropertyStringLen)
			}
		}
	}
	return nil
}

// ValidateFlags validates that operation flags reference valid names.
func ValidateFlags(flags map[string]bool, schema *ModuleSchema) error {
	for name := range flags {
		found := false
		for _, f := range schema.Flags {
			if f.Name == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown flag %q for module %s", name, schema.Path)
		}
	}
	return nil
}
