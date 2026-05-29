package config

import (
	"testing"
)

func TestModuleSchemaGetAction(t *testing.T) {
	schema := &ModuleSchema{
		Path: "/ip/address",
		Actions: map[string]*SchemaAction{
			"add": {
				Name:       "add",
				Parameters: []string{"address", "interface"},
				Validators: []string{"duplicate_ip_per_interface"},
			},
		},
	}

	action, ok := schema.GetAction("add")
	if !ok {
		t.Fatal("expected to find 'add' action")
	}
	if action.Name != "add" {
		t.Errorf("expected name 'add', got %q", action.Name)
	}
	if len(action.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(action.Parameters))
	}

	_, ok = schema.GetAction("nonexistent")
	if ok {
		t.Error("expected not to find non-existent action")
	}
}

func TestModuleSchemaGetProperty(t *testing.T) {
	schema := &ModuleSchema{
		Path: "/ip/address",
		Schema: map[string]*SchemaProperty{
			"address": {
				Name:     "address",
				Type:     SchemaString,
				Required: true,
			},
		},
	}

	prop, ok := schema.GetProperty("address")
	if !ok {
		t.Fatal("expected to find 'address' property")
	}
	if prop.Type != SchemaString {
		t.Errorf("expected type string, got %v", prop.Type)
	}
	if !prop.Required {
		t.Error("expected required to be true")
	}

	_, ok = schema.GetProperty("nonexistent")
	if ok {
		t.Error("expected not to find non-existent property")
	}
}

func TestSchemaPropertyTypes(t *testing.T) {
	tests := []struct {
		propType SchemaPropertyType
		expected string
	}{
		{SchemaString, "string"},
		{SchemaInteger, "integer"},
		{SchemaBoolean, "bool"},
		{SchemaIPAddr, "ipAddr"},
		{SchemaMACAddr, "macAddr"},
		{SchemaIPPrefix, "ipPrefix"},
		{SchemaEnum, "enum"},
		{SchemaInterface, "interface_enum"},
		{SchemaComposite, "composite_arg"},
		{SchemaCompositeIP, "composite_ip"},
	}

	for _, tt := range tests {
		if string(tt.propType) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.propType))
		}
	}
}

func TestSchemaActionFields(t *testing.T) {
	action := &SchemaAction{
		Name:        "add",
		Parameters:  []string{"address", "interface"},
		Validators:  []string{"entry_exists"},
		FlagsSet:    []string{"disabled"},
		Description: "Add a new entry",
	}

	if action.Name != "add" {
		t.Errorf("expected Name 'add', got %q", action.Name)
	}
	if len(action.Parameters) != 2 {
		t.Errorf("expected 2 parameters, got %d", len(action.Parameters))
	}
	if len(action.Validators) != 1 {
		t.Errorf("expected 1 validator, got %d", len(action.Validators))
	}
	if len(action.FlagsSet) != 1 {
		t.Errorf("expected 1 flag set, got %d", len(action.FlagsSet))
	}
	if action.Description != "Add a new entry" {
		t.Errorf("expected Description 'Add a new entry', got %q", action.Description)
	}
}

func TestSchemaFlagFields(t *testing.T) {
	flag := SchemaFlag{
		Letter:      "X",
		Name:        "disabled",
		Description: "Entry is disabled",
	}

	if flag.Letter != "X" {
		t.Errorf("expected Letter 'X', got %q", flag.Letter)
	}
	if flag.Name != "disabled" {
		t.Errorf("expected Name 'disabled', got %q", flag.Name)
	}
}

func TestFullModuleSchema(t *testing.T) {
	schema := &ModuleSchema{
		Path:        "/ip/address",
		Type:        "list",
		Title:       "IP Addresses",
		Description: "Manages IPv4 addresses",
		Flags: []SchemaFlag{
			{Letter: "X", Name: "disabled"},
		},
		Schema: map[string]*SchemaProperty{
			"address": {Type: SchemaString, Required: true},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"address"}},
		},
		Defaults: map[string]interface{}{
			"comment": "",
		},
		Constraints: map[string]string{
			"duplicate_ip_per_interface": "no duplicates",
		},
	}

	if schema.Path != "/ip/address" {
		t.Errorf("expected path '/ip/address', got %q", schema.Path)
	}
	if schema.Type != "list" {
		t.Errorf("expected type 'list', got %q", schema.Type)
	}
	if len(schema.Flags) != 1 {
		t.Errorf("expected 1 flag, got %d", len(schema.Flags))
	}
	if len(schema.Schema) != 1 {
		t.Errorf("expected 1 schema property, got %d", len(schema.Schema))
	}
	if len(schema.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(schema.Actions))
	}
	if len(schema.Defaults) != 1 {
		t.Errorf("expected 1 default, got %d", len(schema.Defaults))
	}
	comment, ok := schema.Defaults["comment"]
	if !ok || comment != "" {
		t.Errorf("expected default comment '', got %v", comment)
	}
}

func TestNewOperation(t *testing.T) {
	op := NewOperation(OpAdd, "/ip/address")
	if op.Type != OpAdd {
		t.Errorf("expected OpAdd, got %v", op.Type)
	}
	if op.Path != "/ip/address" {
		t.Errorf("expected path '/ip/address', got %q", op.Path)
	}
	if len(op.Properties) != 0 {
		t.Errorf("expected empty properties, got %d", len(op.Properties))
	}
	if len(op.Flags) != 0 {
		t.Errorf("expected empty flags, got %d", len(op.Flags))
	}
}

func TestOperationTypes(t *testing.T) {
	if OpAdd != "add" {
		t.Errorf("expected 'add', got %q", OpAdd)
	}
	if OpSet != "set" {
		t.Errorf("expected 'set', got %q", OpSet)
	}
	if OpRemove != "remove" {
		t.Errorf("expected 'remove', got %q", OpRemove)
	}
	if OpPrint != "print" {
		t.Errorf("expected 'print', got %q", OpPrint)
	}
	if OpExport != "export" {
		t.Errorf("expected 'export', got %q", OpExport)
	}
	if OpDisable != "disable" {
		t.Errorf("expected 'disable', got %q", OpDisable)
	}
	if OpEnable != "enable" {
		t.Errorf("expected 'enable', got %q", OpEnable)
	}
	if OpMove != "move" {
		t.Errorf("expected 'move', got %q", OpMove)
	}
}

func TestOperationProperties(t *testing.T) {
	op := NewOperation(OpAdd, "/ip/address")
	op.Properties["address"] = "192.168.1.1/24"
	op.Properties["interface"] = "ether1"
	op.EntryID = "test-id"

	if op.Properties["address"] != "192.168.1.1/24" {
		t.Errorf("expected address property, got %v", op.Properties["address"])
	}
	if op.EntryID != "test-id" {
		t.Errorf("expected entry ID 'test-id', got %q", op.EntryID)
	}
}

func TestModuleRegistry(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	if mm == nil {
		t.Fatal("expected non-nil ModuleManager")
	}
	if len(mm.Modules) != 0 {
		t.Errorf("expected empty modules, got %d", len(mm.Modules))
	}
}

func TestRegisterAndExecuteAdd(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{
		Path: "/ip/address",
		Type: "list",
		Schema: map[string]*SchemaProperty{
			"address":   {Name: "address", Type: SchemaString},
			"interface": {Name: "interface", Type: SchemaString},
			"comment":   {Name: "comment", Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"address", "interface"}},
		},
		Defaults: map[string]interface{}{
			"comment": "",
		},
	}

	err := mm.RegisterModule(schema)
	if err != nil {
		t.Fatalf("unexpected error registering module: %v", err)
	}

	// Execute add operation
	op := NewOperation(OpAdd, "/ip/address")
	op.Properties["address"] = "192.168.1.1/24"
	op.Properties["interface"] = "ether1"

	err = mm.ExecuteOperation(op)
	if err != nil {
		t.Fatalf("unexpected error executing add: %v", err)
	}

	// Verify entry was added
	entries, err := ct.GetEntries("/ip/address")
	if err != nil {
		t.Fatalf("unexpected error getting entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].GetString("address") != "192.168.1.1/24" {
		t.Errorf("expected address '192.168.1.1/24', got %q", entries[0].GetString("address"))
	}
	if entries[0].GetString("interface") != "ether1" {
		t.Errorf("expected interface 'ether1', got %q", entries[0].GetString("interface"))
	}
	if entries[0].GetString("comment") != "" {
		t.Errorf("expected empty comment, got %q", entries[0].GetString("comment"))
	}
}

func TestRegisterAndExecuteRemove(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{
		Path: "/ip/address",
		Type: "list",
		Schema: map[string]*SchemaProperty{
			"address": {Name: "address", Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add":    {Name: "add", Parameters: []string{"address"}},
			"remove": {Name: "remove", Parameters: []string{"numbers"}, Validators: []string{"entry_exists"}},
		},
	}

	mm.RegisterModule(schema)

	// Add entry
	opAdd := NewOperation(OpAdd, "/ip/address")
	opAdd.Properties["address"] = "10.0.0.1/8"
	mm.ExecuteOperation(opAdd)

	// Get the entry's ID
	entries, _ := ct.GetEntries("/ip/address")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Remove entry
	opRemove := NewOperation(OpRemove, "/ip/address")
	opRemove.EntryID = entries[0].ID
	err := mm.ExecuteOperation(opRemove)
	if err != nil {
		t.Fatalf("unexpected error removing entry: %v", err)
	}

	entries, _ = ct.GetEntries("/ip/address")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after removal, got %d", len(entries))
	}
}

func TestExecuteOperationNoModule(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	op := NewOperation(OpAdd, "/nonexistent")
	err := mm.ExecuteOperation(op)
	if err == nil {
		t.Error("expected error for unregistered module")
	}
}

func TestExecuteOperationUnsupportedAction(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{
		Path:    "/test",
		Type:    "list",
		Actions: map[string]*SchemaAction{},
	}
	mm.RegisterModule(schema)

	op := NewOperation(OpAdd, "/test")
	err := mm.ExecuteOperation(op)
	if err == nil {
		t.Error("expected error for unsupported action")
	}
}

func TestDuplicateModuleRegistration(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{Path: "/test", Type: "list"}
	err := mm.RegisterModule(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mm.RegisterModule(schema)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestGetSchema(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{Path: "/test", Type: "list", Title: "Test"}
	mm.RegisterModule(schema)

	got, ok := mm.GetSchema("/test")
	if !ok {
		t.Fatal("expected to find schema")
	}
	if got.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", got.Title)
	}

	_, ok = mm.GetSchema("/nonexistent")
	if ok {
		t.Error("expected not to find non-existent schema")
	}
}

func TestDisabledAndEnable(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mm := NewModuleManager(ct, vr)

	schema := &ModuleSchema{
		Path: "/test",
		Type: "list",
		Schema: map[string]*SchemaProperty{
			"name": {Name: "name", Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add":     {Name: "add", Parameters: []string{"name"}},
			"disable": {Name: "disable", Parameters: []string{"numbers"}, Validators: []string{"entry_exists"}},
			"enable":  {Name: "enable", Parameters: []string{"numbers"}, Validators: []string{"entry_exists"}},
		},
	}
	mm.RegisterModule(schema)

	opAdd := NewOperation(OpAdd, "/test")
	opAdd.Properties["name"] = "test-entry"
	mm.ExecuteOperation(opAdd)

	entries, _ := ct.GetEntries("/test")
	entryID := entries[0].ID

	// Disable
	opDisable := NewOperation(OpDisable, "/test")
	opDisable.EntryID = entryID
	err := mm.ExecuteOperation(opDisable)
	if err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}

	entry, _ := ct.GetEntryByID("/test", entryID)
	if !entry.Disabled {
		t.Error("expected entry to be disabled")
	}

	// Enable
	opEnable := NewOperation(OpEnable, "/test")
	opEnable.EntryID = entryID
	err = mm.ExecuteOperation(opEnable)
	if err != nil {
		t.Fatalf("unexpected error enabling: %v", err)
	}

	entry, _ = ct.GetEntryByID("/test", entryID)
	if entry.Disabled {
		t.Error("expected entry to be enabled")
	}
}
