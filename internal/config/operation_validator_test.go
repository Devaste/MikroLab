package config

import (
	"testing"
)

func TestValidateOperation(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name":    {Type: SchemaString, Required: true},
			"address": {Type: SchemaIPAddr},
			"enabled": {Type: SchemaBoolean},
			"comment": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {
				Name:       "add",
				Parameters: []string{"name", "address", "comment"},
				Validators: []string{},
			},
			"remove": {
				Name:       "remove",
				Parameters: []string{"numbers"},
				Validators: []string{"entry_exists"},
			},
		},
		Defaults: map[string]interface{}{
			"comment": "",
		},
		Flags: []SchemaFlag{
			{Letter: "X", Name: "disabled"},
		},
	}

	if err := mm.RegisterModule(schema); err != nil {
		t.Fatalf("unexpected error registering module: %v", err)
	}

	t.Run("valid add operation", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "  test-entry  "
		op.Properties["address"] = "192.168.1.1"
		op.Properties["comment"] = "test comment"

		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated == nil {
			t.Fatal("expected non-nil validated operation")
		}
		// Verify sanitization: whitespace trimmed
		if validated.Properties["name"] != "test-entry" {
			t.Errorf("expected sanitized name 'test-entry', got %q", validated.Properties["name"])
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		op := NewOperation(OpAdd, "/nonexistent")
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for non-existent path")
		}
	})

	t.Run("unsupported action", func(t *testing.T) {
		op := NewOperation(OpPrint, "/test")
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for unsupported action")
		}
	})

	t.Run("undefined property", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["nonexistent"] = "value"
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for undefined property")
		}
	})

	t.Run("invalid IP address", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		op.Properties["address"] = "not-an-ip"
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for invalid IP address")
		}
	})

	t.Run("invalid boolean value", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		op.Properties["enabled"] = "maybe"
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for invalid boolean value")
		}
	})

	t.Run("missing required property", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["address"] = "192.168.1.1"
		// name is required but not provided
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for missing required property")
		}
	})

	t.Run("required property satisfied by default", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		// comment has a default, so should pass even if not provided
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated == nil {
			t.Fatal("expected non-nil validated operation")
		}
	})

	t.Run("valid numbers", func(t *testing.T) {
		op := NewOperation(OpRemove, "/test")
		op.Numbers = []string{"0", "1", "2"}
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated == nil {
			t.Fatal("expected non-nil validated operation")
		}
	})

	t.Run("invalid numbers", func(t *testing.T) {
		op := NewOperation(OpRemove, "/test")
		op.Numbers = []string{"abc"}
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for invalid numbers")
		}
	})

	t.Run("valid flags", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		op.Flags["disabled"] = true
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated == nil {
			t.Fatal("expected non-nil validated operation")
		}
	})

	t.Run("invalid flags", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		op.Flags["nonexistent"] = true
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for invalid flags")
		}
	})

	t.Run("valid where clause", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		op.Where = map[string]interface{}{"name": "test-entry"}
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated == nil {
			t.Fatal("expected non-nil validated operation")
		}
	})

	t.Run("where with non-existent property", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		op.Where = map[string]interface{}{"nonexistent": "value"}
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for where with non-existent property")
		}
	})

	t.Run("invalid entry ID", func(t *testing.T) {
		op := NewOperation(OpRemove, "/test")
		op.EntryID = "not-a-uuid"
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Fatal("expected error for invalid entry ID")
		}
	})

	t.Run("valid entry ID", func(t *testing.T) {
		op := NewOperation(OpRemove, "/test")
		op.EntryID = "550e8400-e29b-41d4-a716-446655440000"
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated.EntryID != "550e8400-e29b-41d4-a716-446655440000" {
			t.Errorf("expected entry ID to be preserved, got %q", validated.EntryID)
		}
	})
}

func TestValidateOperationInjectionAttempts(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name":        {Type: SchemaString, Required: true},
			"description": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name", "description"}},
		},
		Defaults: map[string]interface{}{
			"description": "",
		},
	}

	if err := mm.RegisterModule(schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("control chars stripped from property", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test\x00\x01\x02"
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated.Properties["name"] != "test" {
			t.Errorf("expected sanitized name 'test', got %q", validated.Properties["name"])
		}
	})

	t.Run("whitespace trimmed from property", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "  test-name  "
		validated, err := ValidateOperation(op, mm)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if validated.Properties["name"] != "test-name" {
			t.Errorf("expected trimmed name 'test-name', got %q", validated.Properties["name"])
		}
	})

	t.Run("property type enforcement", func(t *testing.T) {
		op := NewOperation(OpAdd, "/test")
		op.Properties["name"] = "test"
		// description is SchemaString but we pass integer — should be coerced or error
		op.Properties["description"] = 42
		// CoercePropertyValue for SchemaString expects string, so this should error
		_, err := ValidateOperation(op, mm)
		if err == nil {
			t.Error("expected error for non-string value to string property")
		}
	})
}
