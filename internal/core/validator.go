package core

import "fmt"

// ValidationError represents a single validation failure.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", ve.Field, ve.Message)
}

// ValidationResult holds all validation errors from a check.
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
	return vr.Errors[0].Error()
}

// ValidatorFunc is a pluggable validation function.
// It receives the sanitised property map and (for update operations) the
// existing entry ID. The entry ID is empty for add operations.
// It returns a ValidationResult; if HasErrors() is true the operation
// should be aborted.
type ValidatorFunc func(props map[string]interface{}, entries []Entry, ifaceChecker InterfaceChecker) *ValidationResult

// InterfaceChecker provides interface name validation.
// Modules that reference interfaces (e.g., /ip/address) use this to
// verify that a given interface name exists in the system.
type InterfaceChecker interface {
	// InterfaceExists returns true if the named interface is registered.
	InterfaceExists(name string) bool

	// ListInterfaces returns all known interface names.
	ListInterfaces() []string
}
