package ip_address

import (
	"fmt"
	"net"
	"strings"

	"github.com/Devaste/MikroLab/internal/core"
)

// validatorRegistry maps validator names to their implementation.
// This allows the IPAddressModule to look up validators by name from
// the schema definition (e.g., "duplicate_ip_per_interface").
type validatorRegistry map[string]core.ValidatorFunc

// builtinValidators returns all validators referenced by the ip_address schema.
func builtinValidators() validatorRegistry {
	return validatorRegistry{
		"duplicate_ip_per_interface": validateDuplicateIPPerInterface,
		"interface_exists":           validateInterfaceExists,
		"valid_netmask":              validateValidNetmask,
		"ip_not_in_reserved_range":   validateIPNotInReservedRange,
		"not_dynamic":                validateNotDynamic,
		"entry_exists":               validateEntryExists,
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
			return fmt.Errorf("ip_address: unknown validator %q", name)
		}
		result := fn(props, entries, checker)
		if result.HasErrors() {
			return fmt.Errorf("ip_address: %v", result.Errors[0])
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Validator: duplicate_ip_per_interface
// Schema constraint: "Cannot add the same IP address on the same interface."
// ---------------------------------------------------------------------------

func validateDuplicateIPPerInterface(props map[string]interface{}, entries []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	result := &core.ValidationResult{}
	address, ok := props["address"]
	if !ok {
		return result
	}
	addrStr, ok := address.(string)
	if !ok || addrStr == "" {
		return result
	}

	// If the operation doesn't include an interface, skip (required-property
	// validation elsewhere will catch it).
	ifaceVal, ok := props["interface"]
	if !ok {
		return result
	}
	ifaceStr, ok := ifaceVal.(string)
	if !ok || ifaceStr == "" {
		return result
	}

	for _, existing := range entries {
		existingAddr, _ := existing.Property("address")
		existingIface, _ := existing.Property("interface")
		if existingAddr == addrStr && existingIface == ifaceStr {
			result.Errors = append(result.Errors, core.ValidationError{
				Field:   "address",
				Message: fmt.Sprintf("duplicate IP %q on interface %q", addrStr, ifaceStr),
			})
			return result
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Validator: interface_exists
// Schema constraint: "Interface must exist in /interface."
// ---------------------------------------------------------------------------

func validateInterfaceExists(props map[string]interface{}, _ []core.Entry, checker core.InterfaceChecker) *core.ValidationResult {
	result := &core.ValidationResult{}
	ifaceVal, ok := props["interface"]
	if !ok {
		return result
	}
	ifaceStr, ok := ifaceVal.(string)
	if !ok || ifaceStr == "" {
		return result
	}
	if checker == nil || !checker.InterfaceExists(ifaceStr) {
		result.Errors = append(result.Errors, core.ValidationError{
			Field:   "interface",
			Message: fmt.Sprintf("interface %q does not exist", ifaceStr),
		})
	}
	return result
}

// ---------------------------------------------------------------------------
// Validator: valid_netmask
// Schema constraint: "Netmask must be between /0 and /32."
// ---------------------------------------------------------------------------

func validateValidNetmask(props map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	result := &core.ValidationResult{}
	address, ok := props["address"]
	if !ok {
		return result
	}
	addrStr, ok := address.(string)
	if !ok || addrStr == "" {
		return result
	}

	parts := strings.Split(addrStr, "/")
	if len(parts) != 2 {
		result.Errors = append(result.Errors, core.ValidationError{
			Field:   "address",
			Message: fmt.Sprintf("invalid CIDR notation: %q (missing prefix length)", addrStr),
		})
		return result
	}

	_, ipNet, err := net.ParseCIDR(addrStr)
	if err != nil {
		result.Errors = append(result.Errors, core.ValidationError{
			Field:   "address",
			Message: fmt.Sprintf("invalid CIDR: %v", err),
		})
		return result
	}

	ones, bits := ipNet.Mask.Size()
	if ones < 0 || ones > 32 || bits != 32 {
		result.Errors = append(result.Errors, core.ValidationError{
			Field:   "address",
			Message: "netmask must be between /0 and /32",
		})
	}
	return result
}

// ---------------------------------------------------------------------------
// Validator: ip_not_in_reserved_range
// Schema constraint: "Reserved IP ranges (e.g., 127.0.0.0/8, 0.0.0.0/8)
// cannot be assigned."
// ---------------------------------------------------------------------------

func validateIPNotInReservedRange(props map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	result := &core.ValidationResult{}
	address, ok := props["address"]
	if !ok {
		return result
	}
	addrStr, ok := address.(string)
	if !ok || addrStr == "" {
		return result
	}

	ip := net.ParseIP(strings.Split(addrStr, "/")[0])
	if ip == nil {
		return result
	}

	reserved := []string{
		"0.0.0.0/8",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"224.0.0.0/4",
		"240.0.0.0/4",
	}

	for _, r := range reserved {
		_, cidr, err := net.ParseCIDR(r)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			result.Errors = append(result.Errors, core.ValidationError{
				Field:   "address",
				Message: fmt.Sprintf("IP %s is in reserved range %s", ip, r),
			})
			return result
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Validator: not_dynamic
// Constraint from the "remove" action.
// ---------------------------------------------------------------------------

func validateNotDynamic(props map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	// This is checked at the Remove() level by inspecting the stored entry
	// directly. The validator is registered for completeness.
	return &core.ValidationResult{}
}

// ---------------------------------------------------------------------------
// Validator: entry_exists
// Constraint from "set", "remove", "disable", "enable" actions.
// This is checked at the operation level (entry ID must resolve).
// ---------------------------------------------------------------------------

func validateEntryExists(_ map[string]interface{}, _ []core.Entry, _ core.InterfaceChecker) *core.ValidationResult {
	// Checked by the caller via Get() before mutating. Registered for
	// schema completeness.
	return &core.ValidationResult{}
}
