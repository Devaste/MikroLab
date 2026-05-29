package route

import (
	"fmt"
	"net"
	"strings"

	"github.com/Devaste/MikroLab/internal/core"
)

// validatorRegistry maps validator names to their implementation.
// This allows the RouteModule to look up validators by name from
// the schema definition (e.g., "valid_dst", "valid_gateway").
type validatorRegistry map[string]core.ValidatorFunc

// builtinValidators returns all validators referenced by the route schema.
func builtinValidators() validatorRegistry {
	return validatorRegistry{
		"valid_dst":     validateValidDst,
		"valid_gateway": validateValidGateway,
		"not_dynamic":   validateNotDynamic,
		"entry_exists":  validateEntryExists,
	}
}

// runValidators executes the named validators sequentially.
// If any validator returns errors, the first error is returned immediately
// (fail-fast behaviour matching RouterOS 7 semantics).
func runValidators(
	names []string,
	props map[string]interface{},
	entries []core.Entry,
	checker core.InterfaceChecker,
	reg validatorRegistry,
) error {
	for _, name := range names {
		fn, ok := reg[name]
		if !ok {
			return fmt.Errorf("route: unknown validator %q", name)
		}
		result := fn(props, entries, checker)
		if result.HasErrors() {
			return fmt.Errorf("route: %v", result.Errors[0])
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Validator: valid_dst
// Schema constraint: "dst-address must be a valid CIDR notation."
// Additionally allows 0.0.0.0/0 (default route).
// ---------------------------------------------------------------------------

func validateValidDst(props map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	result := &core.ValidationResult{}

	dstRaw, ok := props["dst-address"]
	if !ok {
		return result
	}
	dst, ok := dstRaw.(string)
	if !ok || strings.TrimSpace(dst) == "" {
		return result
	}
	dst = strings.TrimSpace(dst)

	_, _, err := net.ParseCIDR(dst)
	if err != nil {
		result.Errors = append(result.Errors, core.ValidationError{
			Field:   "dst-address",
			Message: fmt.Sprintf("invalid dst-address %q: must be valid CIDR notation", dst),
		})
		return result
	}

	// Check that prefix length is between 0 and 32 for IPv4
	_, cidr, _ := net.ParseCIDR(dst)
	ones, bits := cidr.Mask.Size()
	if bits != 32 || ones < 0 || ones > 32 {
		result.Errors = append(result.Errors, core.ValidationError{
			Field:   "dst-address",
			Message: fmt.Sprintf("invalid dst-address %q: prefix must be between /0 and /32", dst),
		})
	}

	return result
}

// ---------------------------------------------------------------------------
// Validator: valid_gateway
// Schema constraint: "gateway must be a valid IP address or existing
// interface name."
// ---------------------------------------------------------------------------

func validateValidGateway(props map[string]interface{}, _ []core.Entry, checker core.InterfaceChecker) *core.ValidationResult {
	result := &core.ValidationResult{}

	gwRaw, ok := props["gateway"]
	if !ok {
		return result
	}
	gw, ok := gwRaw.(string)
	if !ok || strings.TrimSpace(gw) == "" {
		return result
	}
	gw = strings.TrimSpace(gw)

	// Skip gateway validation for blackhole routes
	if bhRaw, hasBH := props["blackhole"]; hasBH {
		if bh, ok := bhRaw.(bool); ok && bh {
			return result
		}
	}

	// Check if it's a valid IP address
	if net.ParseIP(gw) != nil {
		return result
	}

	// Check if it's an existing interface name
	if checker != nil && checker.InterfaceExists(gw) {
		return result
	}

	result.Errors = append(result.Errors, core.ValidationError{
		Field:   "gateway",
		Message: fmt.Sprintf("invalid gateway %q: must be a valid IP address or existing interface name", gw),
	})
	return result
}

// ---------------------------------------------------------------------------
// Validator: not_dynamic
// Schema constraint: "Cannot remove or modify a dynamic route."
// The actual check is done in the Set/Remove methods because the validator
// signature does not carry the target entry ID. This validator is registered
// for schema completeness.
// ---------------------------------------------------------------------------

func validateNotDynamic(_ map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	return &core.ValidationResult{}
}

// ---------------------------------------------------------------------------
// Validator: entry_exists
// Schema constraint: entry must exist for "set", "remove" operations.
// Checked by the caller via Get() before mutating. Registered for
// schema completeness.
// ---------------------------------------------------------------------------

func validateEntryExists(_ map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	return &core.ValidationResult{}
}
