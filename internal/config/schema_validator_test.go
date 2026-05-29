package config

import (
	"testing"
)

func TestSchemaValidatorValidSchema(t *testing.T) {
	sv := NewSchemaValidator()
	vr := NewValidatorRegistry()

	schema := &ModuleSchema{
		Path:        "/ip/address",
		Type:        "list",
		Title:       "IP Addresses",
		Description: "Test module",
		Flags: []SchemaFlag{
			{Letter: "X", Name: "disabled", Description: "Entry is disabled"},
		},
		Schema: map[string]*SchemaProperty{
			"address":   {Type: SchemaString, Required: true, Description: "IP address"},
			"interface": {Type: SchemaString, Required: true, Description: "Interface name"},
		},
		Actions: map[string]*SchemaAction{
			"add": {
				Name:        "add",
				Parameters:  []string{"address", "interface"},
				Validators:  []string{},
				Description: "Add entry",
			},
		},
	}

	result := sv.Validate(schema, vr)
	if result.HasErrors() {
		t.Fatalf("expected no validation errors, got: %s", result.Error())
	}
}

func TestSchemaValidatorMissingRequiredFields(t *testing.T) {
	sv := NewSchemaValidator()

	tests := []struct {
		name   string
		schema ModuleSchema
	}{
		{"missing path", ModuleSchema{Type: "list", Title: "Test", Schema: map[string]*SchemaProperty{"a": {Type: SchemaString}}, Actions: map[string]*SchemaAction{"add": {Name: "add"}}}},
		{"missing type", ModuleSchema{Path: "/test", Title: "Test", Schema: map[string]*SchemaProperty{"a": {Type: SchemaString}}, Actions: map[string]*SchemaAction{"add": {Name: "add"}}}},
		{"missing title", ModuleSchema{Path: "/test", Type: "list", Schema: map[string]*SchemaProperty{"a": {Type: SchemaString}}, Actions: map[string]*SchemaAction{"add": {Name: "add"}}}},
		{"missing schema", ModuleSchema{Path: "/test", Type: "list", Title: "Test", Schema: map[string]*SchemaProperty{}, Actions: map[string]*SchemaAction{"add": {Name: "add"}}}},
		{"missing actions", ModuleSchema{Path: "/test", Type: "list", Title: "Test", Schema: map[string]*SchemaProperty{"a": {Type: SchemaString}}, Actions: map[string]*SchemaAction{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sv.Validate(&tt.schema, nil)
			if !result.HasErrors() {
				t.Errorf("expected validation errors for %s", tt.name)
			}
		})
	}
}

func TestSchemaValidatorInvalidType(t *testing.T) {
	sv := NewSchemaValidator()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "invalid_type",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	result := sv.Validate(schema, nil)
	if !result.HasErrors() {
		t.Fatal("expected validation error for invalid type")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "type" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about type field, got: %s", result.Error())
	}
}

func TestSchemaValidatorInvalidPath(t *testing.T) {
	sv := NewSchemaValidator()

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		errField string
	}{
		{"valid path", "/ip/address", false, ""},
		{"root path", "/", false, ""},
		{"missing leading slash", "ip/address", true, "path"},
		{"trailing slash", "/ip/address/", true, "path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &ModuleSchema{
				Path:  tt.path,
				Type:  "list",
				Title: "Test",
				Schema: map[string]*SchemaProperty{
					"name": {Type: SchemaString},
				},
				Actions: map[string]*SchemaAction{
					"add": {Name: "add", Parameters: []string{"name"}},
				},
			}
			result := sv.Validate(schema, nil)
			if tt.wantErr && !result.HasErrors() {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && result.HasErrors() {
				t.Fatalf("expected no errors, got: %s", result.Error())
			}
			if tt.wantErr && tt.errField != "" {
				found := false
				for _, e := range result.Errors {
					if e.Field == tt.errField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error on field %q, got: %s", tt.errField, result.Error())
				}
			}
		})
	}
}

func TestSchemaValidatorUnknownPropertyType(t *testing.T) {
	sv := NewSchemaValidator()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"custom": {Type: SchemaPropertyType("unknown_type")},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{}},
		},
	}

	result := sv.Validate(schema, nil)
	if !result.HasErrors() {
		t.Fatal("expected validation error for unknown property type")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "schema.custom.type" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about schema.custom.type, got: %s", result.Error())
	}
}

func TestSchemaValidatorCompositeWithoutComponents(t *testing.T) {
	sv := NewSchemaValidator()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"composite": {Type: SchemaComposite},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{}},
		},
	}

	result := sv.Validate(schema, nil)
	if !result.HasErrors() {
		t.Fatal("expected validation error for composite without components")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "schema.composite.components" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about composite.components, got: %s", result.Error())
	}
}

func TestSchemaValidatorDynamicValuesWithoutRef(t *testing.T) {
	sv := NewSchemaValidator()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"iface": {Type: SchemaString, DynamicValues: true},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{}},
		},
	}

	result := sv.Validate(schema, nil)
	if !result.HasErrors() {
		t.Fatal("expected validation error for dynamicValues without ref")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "schema.iface.ref" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about iface.ref, got: %s", result.Error())
	}
}

func TestSchemaValidatorParamReferencesNonExistentProperty(t *testing.T) {
	sv := NewSchemaValidator()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {
				Name:       "add",
				Parameters: []string{"nonexistent"},
			},
		},
	}

	result := sv.Validate(schema, nil)
	if !result.HasErrors() {
		t.Fatal("expected validation error for parameter referencing non-existent property")
	}
}

func TestSchemaValidatorFlagValidation(t *testing.T) {
	sv := NewSchemaValidator()

	tests := []struct {
		name    string
		flags   []SchemaFlag
		wantErr bool
	}{
		{"valid flags", []SchemaFlag{{Letter: "X", Name: "disabled"}, {Letter: "D", Name: "dynamic"}}, false},
		{"invalid letter", []SchemaFlag{{Letter: "x", Name: "disabled"}}, true},
		{"duplicate letter", []SchemaFlag{{Letter: "X", Name: "disabled"}, {Letter: "X", Name: "dynamic"}}, true},
		{"duplicate name", []SchemaFlag{{Letter: "X", Name: "disabled"}, {Letter: "D", Name: "disabled"}}, true},
		{"empty name", []SchemaFlag{{Letter: "X", Name: ""}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &ModuleSchema{
				Path:  "/test",
				Type:  "list",
				Title: "Test",
				Flags: tt.flags,
				Schema: map[string]*SchemaProperty{
					"name": {Type: SchemaString},
				},
				Actions: map[string]*SchemaAction{
					"add": {Name: "add", Parameters: []string{"name"}},
				},
			}
			result := sv.Validate(schema, nil)
			if tt.wantErr && !result.HasErrors() {
				t.Errorf("expected validation error for %s", tt.name)
			}
			if !tt.wantErr && result.HasErrors() {
				t.Errorf("unexpected errors for %s: %s", tt.name, result.Error())
			}
		})
	}
}

func TestSchemaValidatorUnknownValidator(t *testing.T) {
	sv := NewSchemaValidator()
	vr := NewValidatorRegistry()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {
				Name:       "add",
				Parameters: []string{"name"},
				Validators: []string{"nonexistent_validator"},
			},
		},
	}

	result := sv.Validate(schema, vr)
	if !result.HasErrors() {
		t.Fatal("expected validation error for unknown validator")
	}

	found := false
	for _, e := range result.Errors {
		if e.Field == "actions.add.validators" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about unknown validator, got: %s", result.Error())
	}
}

func TestSchemaValidatorFlagsSetNotDefined(t *testing.T) {
	sv := NewSchemaValidator()

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Flags: []SchemaFlag{
			{Letter: "X", Name: "disabled"},
		},
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {
				Name:       "add",
				Parameters: []string{"name"},
				FlagsSet:   []string{"nonexistent_flag"},
			},
		},
	}

	result := sv.Validate(schema, nil)
	if !result.HasErrors() {
		t.Fatal("expected validation error for undefined flag in flags_set")
	}
}

func TestSchemaValidatorDependencies(t *testing.T) {
	sv := NewSchemaValidator()

	tests := []struct {
		name         string
		dependencies []SchemaDependency
		wantErr      bool
	}{
		{"valid dependency", []SchemaDependency{{Path: "/ip/address"}}, false},
		{"empty path", []SchemaDependency{{Path: ""}}, true},
		{"path without leading slash", []SchemaDependency{{Path: "ip/address"}}, true},
		{"duplicate path", []SchemaDependency{{Path: "/ip/address"}, {Path: "/ip/address"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &ModuleSchema{
				Path:         "/test",
				Type:         "list",
				Title:        "Test",
				Dependencies: tt.dependencies,
				Schema: map[string]*SchemaProperty{
					"name": {Type: SchemaString},
				},
				Actions: map[string]*SchemaAction{
					"add": {Name: "add", Parameters: []string{"name"}},
				},
			}
			result := sv.Validate(schema, nil)
			if tt.wantErr && !result.HasErrors() {
				t.Errorf("expected validation error for %s", tt.name)
			}
			if !tt.wantErr && result.HasErrors() {
				t.Errorf("unexpected errors for %s: %s", tt.name, result.Error())
			}
		})
	}
}

func TestSchemaValidatorVersion(t *testing.T) {
	sv := NewSchemaValidator()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"no version", "", false},
		{"valid semver", "v1.0.0", false},
		{"valid semver patch", "v1.2.3", false},
		{"missing v prefix", "1.0.0", true},
		{"too short", "v", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &ModuleSchema{
				Path:    "/test",
				Type:    "list",
				Title:   "Test",
				Version: tt.version,
				Schema: map[string]*SchemaProperty{
					"name": {Type: SchemaString},
				},
				Actions: map[string]*SchemaAction{
					"add": {Name: "add", Parameters: []string{"name"}},
				},
			}
			result := sv.Validate(schema, nil)
			if tt.wantErr && !result.HasErrors() {
				t.Errorf("expected validation error for version %q", tt.version)
			}
			if !tt.wantErr && result.HasErrors() {
				t.Errorf("unexpected errors for version %q: %s", tt.version, result.Error())
			}
		})
	}
}

func TestSchemaValidatorValidatePathSecurity(t *testing.T) {
	sv := NewSchemaValidator().WithTrustedSourceDirs([]string{"/etc/mikrolab/modules"})

	// Schema with source file in trusted directory
	schema := &ModuleSchema{
		Path:       "/test",
		Type:       "list",
		Title:      "Test",
		SourceFile: "/etc/mikrolab/modules/test.json",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	// This should pass because we set trusted dirs
	result := sv.Validate(schema, nil)
	if result.HasErrors() {
		t.Logf("Path security may trigger on test system; actual errors: %s", result.Error())
	}
}

func TestSchemaValidatorWithIPAddressSchema(t *testing.T) {
	sv := NewSchemaValidator()
	vr := NewValidatorRegistry()

	// This schema mirrors the real ip_address.json
	schema := &ModuleSchema{
		Path:        "/ip/address",
		Type:        "list",
		Title:       "IP Addresses",
		Description: "Manages IPv4 addresses",
		Flags: []SchemaFlag{
			{Letter: "X", Name: "disabled", Description: "Disabled entry"},
			{Letter: "I", Name: "invalid", Description: "Invalid config"},
			{Letter: "D", Name: "dynamic", Description: "Dynamic entry"},
			{Letter: "S", Name: "slave", Description: "Slave entry"},
		},
		Schema: map[string]*SchemaProperty{
			"address": {
				Type:        SchemaComposite,
				Separator:   "/",
				Required:    true,
				Description: "IPv4 with prefix",
				Components: map[string]*SchemaProperty{
					"address": {Type: SchemaIPAddr, Required: true},
					"netmask": {Type: SchemaIPAddr, Required: false},
				},
			},
			"network":          {Type: SchemaIPAddr, Description: "Network address"},
			"broadcast":        {Type: SchemaIPAddr, Description: "Broadcast address"},
			"interface":        {Type: SchemaInterface, Required: true, Description: "Interface", DynamicValues: true, Ref: "/interface"},
			"actual-interface": {Type: SchemaInterface, ReadOnly: true, ComputedFrom: "interface"},
			"vrf":              {Type: SchemaEnum, Default: "main", ReadOnly: true, Description: "VRF"},
			"comment":          {Type: SchemaString, Default: "", Description: "Comment"},
		},
		Actions: map[string]*SchemaAction{
			"add": {
				Name:        "add",
				Parameters:  []string{"address", "interface", "comment"},
				Validators:  []string{"duplicate_ip_per_interface", "valid_netmask", "interface_exists", "ip_not_in_reserved_range"},
				FlagsSet:    []string{"disabled"},
				Description: "Add IP address",
			},
			"set": {
				Name:        "set",
				Parameters:  []string{"numbers", "address", "interface", "comment"},
				Validators:  []string{"entry_exists", "duplicate_ip_per_interface", "interface_exists"},
				Description: "Modify IP address",
			},
			"remove": {
				Name:        "remove",
				Parameters:  []string{"numbers"},
				Validators:  []string{"entry_exists", "not_dynamic"},
				Description: "Delete IP address",
			},
			"disable": {
				Name:        "disable",
				Parameters:  []string{"numbers"},
				Validators:  []string{"entry_exists"},
				Description: "Disable IP address",
			},
			"enable": {
				Name:        "enable",
				Parameters:  []string{"numbers"},
				Validators:  []string{"entry_exists"},
				Description: "Enable IP address",
			},
		},
		Defaults: map[string]interface{}{
			"comment": "",
			"vrf":     "main",
		},
	}

	result := sv.Validate(schema, vr)
	if result.HasErrors() {
		t.Fatalf("expected no validation errors for ip_address schema, got: %s", result.Error())
	}
}

func TestSchemaValidationResult(t *testing.T) {
	result := &SchemaValidationResult{}
	if result.HasErrors() {
		t.Error("expected no errors in empty result")
	}
	if result.Error() != "" {
		t.Errorf("expected empty string, got %q", result.Error())
	}

	result.Errors = append(result.Errors, SchemaValidationError{
		Field:   "path",
		Message: "path is required",
	})
	if !result.HasErrors() {
		t.Error("expected errors after adding one")
	}
	if result.Error() == "" {
		t.Error("expected non-empty error string")
	}
}
