package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidationError represents a validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", ve.Field, ve.Message)
}

// ValidationResult holds all validation errors.
type ValidationResult struct {
	Errors []ValidationError `json:"errors"`
}

func (vr *ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

func (vr *ValidationResult) Error() string {
	if !vr.HasErrors() {
		return ""
	}
	msgs := make([]string, len(vr.Errors))
	for i, e := range vr.Errors {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// ValidatorFunc is a function that validates an operation against the tree.
type ValidatorFunc func(operation *Operation, tree *ConfigTree) *ValidationResult

// ValidatorRegistry holds all registered validators.
type ValidatorRegistry struct {
	validators map[string]ValidatorFunc
}

// NewValidatorRegistry creates a new validator registry with built-in validators.
func NewValidatorRegistry() *ValidatorRegistry {
	vr := &ValidatorRegistry{
		validators: make(map[string]ValidatorFunc),
	}
	vr.registerBuiltins()
	return vr
}

// Register adds a validator function by name.
func (vr *ValidatorRegistry) Register(name string, fn ValidatorFunc) {
	vr.validators[name] = fn
}

// Get returns a validator function by name.
func (vr *ValidatorRegistry) Get(name string) (ValidatorFunc, bool) {
	fn, ok := vr.validators[name]
	return fn, ok
}

// Validate runs a list of validators against an operation.
func (vr *ValidatorRegistry) Validate(validatorNames []string, operation *Operation, tree *ConfigTree) *ValidationResult {
	result := &ValidationResult{}
	for _, name := range validatorNames {
		fn, ok := vr.Get(name)
		if !ok {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "_schema",
				Message: fmt.Sprintf("unknown validator: %s", name),
			})
			continue
		}
		r := fn(operation, tree)
		if r.HasErrors() {
			result.Errors = append(result.Errors, r.Errors...)
		}
	}
	return result
}

func (vr *ValidatorRegistry) registerBuiltins() {
	// entry_exists: verifies that the entry specified by .id exists
	vr.Register("entry_exists", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		if op.EntryID == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   ".id",
				Message: "entry ID is required",
			})
			return result
		}
		_, err := tree.GetEntryByID(op.Path, op.EntryID)
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   ".id",
				Message: err.Error(),
			})
		}
		return result
	})

	// not_dynamic: verifies the entry is not dynamic
	vr.Register("not_dynamic", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		if op.EntryID == "" {
			return result
		}
		entry, err := tree.GetEntryByID(op.Path, op.EntryID)
		if err != nil {
			return result
		}
		if entry.Dynamic {
			result.Errors = append(result.Errors, ValidationError{
				Field:   ".id",
				Message: "cannot modify dynamic entry",
			})
		}
		return result
	})

	// duplicate_ip_per_interface: verifies no duplicate IP on same interface
	vr.Register("duplicate_ip_per_interface", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		address, ok := op.Properties["address"]
		if !ok {
			return result
		}
		iface, ok := op.Properties["interface"]
		if !ok {
			return result
		}

		addrStr, ok := address.(string)
		if !ok {
			return result
		}
		ifaceStr, ok := iface.(string)
		if !ok {
			return result
		}

		entries, err := tree.GetEntries(op.Path)
		if err != nil {
			return result
		}

		for _, e := range entries {
			if e.ID == op.EntryID {
				continue // skip the entry being updated
			}
			if e.GetString("address") == addrStr && e.GetString("interface") == ifaceStr {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "address",
					Message: fmt.Sprintf("duplicate IP %s on interface %s", addrStr, ifaceStr),
				})
				return result
			}
		}
		return result
	})

	// valid_netmask: verifies netmask is between /0 and /32
	vr.Register("valid_netmask", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		address, ok := op.Properties["address"]
		if !ok {
			return result
		}
		addrStr, ok := address.(string)
		if !ok {
			return result
		}

		parts := strings.Split(addrStr, "/")
		if len(parts) != 2 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "address",
				Message: fmt.Sprintf("invalid CIDR notation: %s", addrStr),
			})
			return result
		}

		_, ipNet, err := net.ParseCIDR(addrStr)
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "address",
				Message: fmt.Sprintf("invalid CIDR: %s", err.Error()),
			})
			return result
		}

		ones, bits := ipNet.Mask.Size()
		if ones < 0 || ones > 32 || bits != 32 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "address",
				Message: "netmask must be between /0 and /32",
			})
		}
		return result
	})

	// interface_exists: verifies the referenced interface exists
	vr.Register("interface_exists", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		iface, ok := op.Properties["interface"]
		if !ok {
			return result
		}
		ifaceStr, ok := iface.(string)
		if !ok || ifaceStr == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "interface",
				Message: "interface is required",
			})
			return result
		}
		// TODO: check /interface list when implemented
		return result
	})

	// ip_not_in_reserved_range: verifies IP is not in reserved ranges
	vr.Register("ip_not_in_reserved_range", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		address, ok := op.Properties["address"]
		if !ok {
			return result
		}
		addrStr, ok := address.(string)
		if !ok {
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
			_, cidr, _ := net.ParseCIDR(r)
			if cidr.Contains(ip) {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "address",
					Message: fmt.Sprintf("IP %s is in reserved range %s", ip, r),
				})
				return result
			}
		}
		return result
	})

	// duplicate_arp_entry: verifies no duplicate ARP entry
	vr.Register("duplicate_arp_entry", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		address, ok := op.Properties["address"]
		if !ok {
			return result
		}
		mac, ok := op.Properties["mac-address"]
		if !ok {
			return result
		}
		iface, ok := op.Properties["interface"]
		if !ok {
			return result
		}

		addrStr, _ := address.(string)
		macStr, _ := mac.(string)
		ifaceStr, _ := iface.(string)

		entries, err := tree.GetEntries(op.Path)
		if err != nil {
			return result
		}

		for _, e := range entries {
			if e.ID == op.EntryID {
				continue
			}
			if e.GetString("address") == addrStr &&
				e.GetString("mac-address") == macStr &&
				e.GetString("interface") == ifaceStr {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "address",
					Message: fmt.Sprintf("duplicate ARP entry %s/%s on %s", addrStr, macStr, ifaceStr),
				})
				return result
			}
		}
		return result
	})

	// valid_entry_id: validates entry ID format
	vr.Register("valid_entry_id", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		if op.EntryID == "" {
			return result // entry ID may be empty for add operations
		}
		if err := ValidateEntryID(op.EntryID); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   ".id",
				Message: err.Error(),
			})
		}
		return result
	})

	// valid_numbers: validates that number references are well-formed
	vr.Register("valid_numbers", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		if len(op.Numbers) == 0 {
			return result
		}
		if err := ValidateNumbers(op.Numbers); err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "numbers",
				Message: err.Error(),
			})
		}
		return result
	})

	// valid_mac_address: validates MAC format
	vr.Register("valid_mac_address", func(op *Operation, tree *ConfigTree) *ValidationResult {
		result := &ValidationResult{}
		mac, ok := op.Properties["mac-address"]
		if !ok {
			return result
		}
		macStr, ok := mac.(string)
		if !ok {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "mac-address",
				Message: "MAC address must be a string",
			})
			return result
		}

		// Accept formats: 00:11:22:33:44:55, 00-11-22-33-44-55, 0011.2233.4455
		patterns := []string{
			`^([0-9A-Fa-f]{2}[:]){5}[0-9A-Fa-f]{2}$`,
			`^([0-9A-Fa-f]{2}[-]){5}[0-9A-Fa-f]{2}$`,
			`^([0-9A-Fa-f]{4}[.]){2}[0-9A-Fa-f]{4}$`,
		}

		valid := false
		for _, p := range patterns {
			matched, _ := regexp.MatchString(p, macStr)
			if matched {
				valid = true
				break
			}
		}

		if !valid {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "mac-address",
				Message: fmt.Sprintf("invalid MAC address format: %s", macStr),
			})
		}
		return result
	})
}
