package config

import (
	"testing"
)

func setupTestTree(t *testing.T) *ConfigTree {
	t.Helper()
	ct := NewConfigTree()
	ct.EnsurePath("/ip/address", NodeTypeList, "IP Addresses")
	return ct
}

func TestNewValidatorRegistry(t *testing.T) {
	vr := NewValidatorRegistry()
	if vr == nil {
		t.Fatal("expected non-nil ValidatorRegistry")
	}
}

func TestValidatorRegisterAndGet(t *testing.T) {
	vr := NewValidatorRegistry()
	fn := func(op *Operation, tree *ConfigTree) *ValidationResult {
		return &ValidationResult{}
	}
	vr.Register("custom_validator", fn)

	got, ok := vr.Get("custom_validator")
	if !ok {
		t.Fatal("expected to find custom_validator")
	}
	if got == nil {
		t.Fatal("expected non-nil validator function")
	}
}

func TestValidatorEntryExists(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	// Entry doesn't exist yet
	op := NewOperation(OpRemove, "/ip/address")
	op.EntryID = "nonexistent"

	result := vr.Validate([]string{"entry_exists"}, op, ct)
	if !result.HasErrors() {
		t.Error("expected error for non-existent entry")
	}

	// Add an entry and try again
	e := NewEntry("", 0)
	ct.AddEntry("/ip/address", e)

	op.EntryID = e.ID
	result = vr.Validate([]string{"entry_exists"}, op, ct)
	if result.HasErrors() {
		t.Errorf("unexpected validation errors: %v", result)
	}
}

func TestValidatorEntryExistsNoID(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	op := NewOperation(OpRemove, "/ip/address")
	result := vr.Validate([]string{"entry_exists"}, op, ct)
	if !result.HasErrors() {
		t.Error("expected error when entry ID is empty")
	}
}

func TestValidatorNotDynamic(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	// Add dynamic entry
	e := NewEntry("", 0)
	e.Dynamic = true
	ct.AddEntry("/ip/address", e)

	op := NewOperation(OpRemove, "/ip/address")
	op.EntryID = e.ID

	result := vr.Validate([]string{"not_dynamic"}, op, ct)
	if !result.HasErrors() {
		t.Error("expected error for dynamic entry")
	}
}

func TestValidatorNotDynamicStaticEntry(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	e := NewEntry("", 0)
	ct.AddEntry("/ip/address", e)

	op := NewOperation(OpRemove, "/ip/address")
	op.EntryID = e.ID

	result := vr.Validate([]string{"not_dynamic"}, op, ct)
	if result.HasErrors() {
		t.Errorf("unexpected validation errors: %v", result)
	}
}

func TestValidatorDuplicateIPPerInterface(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	// Add first entry
	e1 := NewEntry("", 0)
	e1.Properties["address"] = &PropertyValue{Name: "address", Value: "192.168.1.1/24"}
	e1.Properties["interface"] = &PropertyValue{Name: "interface", Value: "ether1"}
	ct.AddEntry("/ip/address", e1)

	// Try to add duplicate
	op := NewOperation(OpAdd, "/ip/address")
	op.Properties["address"] = "192.168.1.1/24"
	op.Properties["interface"] = "ether1"

	result := vr.Validate([]string{"duplicate_ip_per_interface"}, op, ct)
	if !result.HasErrors() {
		t.Error("expected error for duplicate IP on same interface")
	}
}

func TestValidatorDuplicateIPDifferentInterface(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	e1 := NewEntry("", 0)
	e1.Properties["address"] = &PropertyValue{Name: "address", Value: "192.168.1.1/24"}
	e1.Properties["interface"] = &PropertyValue{Name: "interface", Value: "ether1"}
	ct.AddEntry("/ip/address", e1)

	// Same IP, different interface - should be valid
	op := NewOperation(OpAdd, "/ip/address")
	op.Properties["address"] = "192.168.1.1/24"
	op.Properties["interface"] = "ether2"

	result := vr.Validate([]string{"duplicate_ip_per_interface"}, op, ct)
	if result.HasErrors() {
		t.Errorf("unexpected validation errors for different interface: %v", result)
	}
}

func TestValidatorValidNetmask(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	tests := []struct {
		cidr    string
		isValid bool
	}{
		{"192.168.1.1/24", true},
		{"10.0.0.1/8", true},
		{"172.16.0.1/16", true},
		{"192.168.1.1/0", true},
		{"192.168.1.1/32", true},
		{"invalid", false},
		{"192.168.1.1", false},
		{"192.168.1.1/33", false},
	}

	for _, tt := range tests {
		op := NewOperation(OpAdd, "/ip/address")
		op.Properties["address"] = tt.cidr

		result := vr.Validate([]string{"valid_netmask"}, op, ct)
		if tt.isValid && result.HasErrors() {
			t.Errorf("expected valid for %q, got errors: %v", tt.cidr, result)
		}
		if !tt.isValid && !result.HasErrors() {
			t.Errorf("expected invalid for %q", tt.cidr)
		}
	}
}

func TestValidatorInterfaceExists(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	// With interface specified
	op := NewOperation(OpAdd, "/ip/address")
	op.Properties["interface"] = "ether1"

	result := vr.Validate([]string{"interface_exists"}, op, ct)
	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result)
	}

	// Without interface
	op2 := NewOperation(OpAdd, "/ip/address")
	result = vr.Validate([]string{"interface_exists"}, op2, ct)
	if result.HasErrors() {
		t.Errorf("unexpected errors when interface not provided: %v", result)
	}

	// With empty interface
	op3 := NewOperation(OpAdd, "/ip/address")
	op3.Properties["interface"] = ""
	result = vr.Validate([]string{"interface_exists"}, op3, ct)
	if !result.HasErrors() {
		t.Error("expected error for empty interface")
	}
}

func TestValidatorIPNotInReservedRange(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	tests := []struct {
		address string
		isValid bool
	}{
		{"127.0.0.1/8", false},    // loopback
		{"224.0.0.1/4", false},    // multicast
		{"240.0.0.1/4", false},    // reserved
		{"0.0.0.1/8", false},      // "this network"
		{"169.254.1.1/16", false}, // link-local
		{"8.8.8.8/32", true},      // public
		{"1.1.1.1/32", true},      // public
		{"192.168.1.1/24", true},  // RFC1918, valid for assignment
		{"10.0.0.1/8", true},      // RFC1918, valid for assignment
		{"172.16.0.1/12", true},   // RFC1918, valid for assignment
	}

	for _, tt := range tests {
		op := NewOperation(OpAdd, "/ip/address")
		op.Properties["address"] = tt.address

		result := vr.Validate([]string{"ip_not_in_reserved_range"}, op, ct)
		if tt.isValid && result.HasErrors() {
			t.Errorf("expected valid for %q, got errors: %v", tt.address, result)
		}
		if !tt.isValid && !result.HasErrors() {
			t.Errorf("expected invalid for %q", tt.address)
		}
	}
}

func TestValidatorDuplicateARPEntry(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := NewConfigTree()
	ct.EnsurePath("/ip/arp", NodeTypeList, "ARP")

	// Add first entry
	e1 := NewEntry("", 0)
	e1.Properties["address"] = &PropertyValue{Name: "address", Value: "192.168.1.1"}
	e1.Properties["mac-address"] = &PropertyValue{Name: "mac-address", Value: "00:11:22:33:44:55"}
	e1.Properties["interface"] = &PropertyValue{Name: "interface", Value: "ether1"}
	ct.AddEntry("/ip/arp", e1)

	// Try to add duplicate
	op := NewOperation(OpAdd, "/ip/arp")
	op.Properties["address"] = "192.168.1.1"
	op.Properties["mac-address"] = "00:11:22:33:44:55"
	op.Properties["interface"] = "ether1"

	result := vr.Validate([]string{"duplicate_arp_entry"}, op, ct)
	if !result.HasErrors() {
		t.Error("expected error for duplicate ARP entry")
	}
}

func TestValidatorValidMACAddress(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	tests := []struct {
		mac     string
		isValid bool
	}{
		{"00:11:22:33:44:55", true},
		{"aa:bb:cc:dd:ee:ff", true},
		{"00-11-22-33-44-55", true},
		{"0011.2233.4455", true},
		{"invalid", false},
		{"00:11:22:33:44:GG", false},
		{"00:11:22:33:44", false},
	}

	for _, tt := range tests {
		op := NewOperation(OpAdd, "/ip/arp")
		op.Properties["mac-address"] = tt.mac

		result := vr.Validate([]string{"valid_mac_address"}, op, ct)
		if tt.isValid && result.HasErrors() {
			t.Errorf("expected valid for %q, got errors: %v", tt.mac, result)
		}
		if !tt.isValid && !result.HasErrors() {
			t.Errorf("expected invalid for %q", tt.mac)
		}
	}
}

func TestValidatorUnknownValidator(t *testing.T) {
	vr := NewValidatorRegistry()
	ct := setupTestTree(t)

	op := NewOperation(OpAdd, "/ip/address")
	result := vr.Validate([]string{"nonexistent_validator"}, op, ct)
	if !result.HasErrors() {
		t.Error("expected error for unknown validator")
	}
}

func TestValidationResult(t *testing.T) {
	vr := &ValidationResult{}
	if vr.HasErrors() {
		t.Error("expected no errors initially")
	}
	if vr.Error() != "" {
		t.Errorf("expected empty error string, got %q", vr.Error())
	}

	vr.Errors = append(vr.Errors, ValidationError{Field: "test", Message: "error message"})
	if !vr.HasErrors() {
		t.Error("expected HasErrors to be true")
	}
	if vr.Error() == "" {
		t.Error("expected non-empty error string")
	}
}

func TestValidationError(t *testing.T) {
	ve := ValidationError{Field: "address", Message: "invalid address"}
	errStr := ve.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
}
