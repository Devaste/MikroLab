package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewModuleRegistry(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	if mr == nil {
		t.Fatal("expected non-nil ModuleRegistry")
	}
	if mr.GetManager() == nil {
		t.Fatal("expected non-nil ModuleManager")
	}

	modules := mr.ListModules()
	if len(modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(modules))
	}
}

func TestModuleRegistryRegisterModule(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	err := mr.RegisterModule(schema)
	if err != nil {
		t.Fatalf("unexpected error registering module: %v", err)
	}

	modules := mr.ListModules()
	if len(modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(modules))
	}
	if modules[0] != "/test" {
		t.Errorf("expected module path /test, got %q", modules[0])
	}

	// Verify we can get it back
	got, ok := mr.GetSchema("/test")
	if !ok {
		t.Fatal("expected to find schema")
	}
	if got.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", got.Title)
	}
}

func TestModuleRegistryRegisterInvalidModule(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	// Missing required fields
	schema := &ModuleSchema{
		Path:  "",
		Type:  "invalid",
		Title: "",
	}

	err := mr.RegisterModule(schema)
	if err == nil {
		t.Fatal("expected error for invalid schema")
	}
}

func TestModuleRegistryDependencyResolution(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	// Register dependency first
	depSchema := &ModuleSchema{
		Path:  "/interface",
		Type:  "list",
		Title: "Interfaces",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}
	if err := mr.RegisterModule(depSchema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Register module that depends on it
	schema := &ModuleSchema{
		Path:  "/ip/address",
		Type:  "list",
		Title: "IP Addresses",
		Dependencies: []SchemaDependency{
			{Path: "/interface"},
		},
		Schema: map[string]*SchemaProperty{
			"address":   {Type: SchemaString},
			"interface": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"address", "interface"}},
		},
	}
	if err := mr.RegisterModule(schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both are registered
	modules := mr.ListModules()
	if len(modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(modules))
	}
}

func TestModuleRegistryUnresolvedDependency(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	schema := &ModuleSchema{
		Path:  "/ip/address",
		Type:  "list",
		Title: "IP Addresses",
		Dependencies: []SchemaDependency{
			{Path: "/nonexistent"},
		},
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	err := mr.RegisterModule(schema)
	if err == nil {
		t.Fatal("expected error for unresolved dependency")
	}
}

func TestModuleRegistryPathOverlap(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	// Register parent module
	parent := &ModuleSchema{
		Path:  "/ip",
		Type:  "directory",
		Title: "IP",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}
	if err := mr.RegisterModule(parent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Register child module — should fail due to overlap
	child := &ModuleSchema{
		Path:  "/ip/address",
		Type:  "list",
		Title: "IP Addresses",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	err := mr.RegisterModule(child)
	if err == nil {
		t.Fatal("expected error for path overlap")
	}
}

func TestModuleRegistryUnloadModule(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}
	mr.RegisterModule(schema)

	// Unload
	err := mr.UnloadModule("/test")
	if err != nil {
		t.Fatalf("unexpected error unloading module: %v", err)
	}

	modules := mr.ListModules()
	if len(modules) != 0 {
		t.Errorf("expected 0 modules after unload, got %d", len(modules))
	}

	// Verify it's gone
	_, ok := mr.GetSchema("/test")
	if ok {
		t.Error("expected schema to be removed")
	}
}

func TestModuleRegistryUnloadWithDependents(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	// Register dependency
	if err := mr.RegisterModule(&ModuleSchema{
		Path:  "/interface",
		Type:  "list",
		Title: "Interfaces",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Register dependent
	if err := mr.RegisterModule(&ModuleSchema{
		Path:  "/ip/address",
		Type:  "list",
		Title: "IP Addresses",
		Dependencies: []SchemaDependency{
			{Path: "/interface"},
		},
		Schema: map[string]*SchemaProperty{
			"address": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"address"}},
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to unload dependency — should fail
	err := mr.UnloadModule("/interface")
	if err == nil {
		t.Fatal("expected error unloading module with dependents")
	}
}

func TestModuleRegistryLoadModuleFile(t *testing.T) {
	// Create a temporary module file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_module.json")

	content := `{
		"path": "/test",
		"type": "list",
		"title": "Test",
		"schema": {
			"name": { "type": "string", "required": true }
		},
		"actions": {
			"add": { "name": "add", "parameters": ["name"] }
		}
	}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)
	mr.WithTrustedDirs([]string{dir})

	schema, err := mr.LoadModuleFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error loading module file: %v", err)
	}
	if schema.Path != "/test" {
		t.Errorf("expected path /test, got %q", schema.Path)
	}
	if schema.SourceFile == "" {
		t.Error("expected SourceFile to be set")
	}
}

func TestModuleRegistryLoadModuleFileUntrusted(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	// Register a trusted dir
	mr.WithTrustedDirs([]string{"/etc/mikrolab/modules"})

	schema := &ModuleSchema{
		Path:       "/test",
		Type:       "list",
		Title:      "Test",
		SourceFile: "/tmp/untrusted/test.json",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	err := mr.RegisterModule(schema)
	if err == nil {
		t.Fatal("expected error for untrusted source file")
	}
}

func TestModuleRegistryLoadModulesFromDir(t *testing.T) {
	dir := t.TempDir()

	// Create two module files
	mod1 := `{
		"path": "/module1",
		"type": "list",
		"title": "Module 1",
		"schema": { "name": { "type": "string" } },
		"actions": { "add": { "name": "add", "parameters": ["name"] } }
	}`
	mod2 := `{
		"path": "/module2",
		"type": "list",
		"title": "Module 2",
		"schema": { "value": { "type": "integer" } },
		"actions": { "add": { "name": "add", "parameters": ["value"] } }
	}`

	if err := os.WriteFile(filepath.Join(dir, "mod1.json"), []byte(mod1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mod2.json"), []byte(mod2), 0644); err != nil {
		t.Fatal(err)
	}

	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)
	mr.WithTrustedDirs([]string{dir})

	err := mr.LoadModulesFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error loading modules from dir: %v", err)
	}

	modules := mr.ListModules()
	if len(modules) != 2 {
		t.Errorf("expected 2 modules, got %d", len(modules))
	}

	byDir := mr.ListModulesByDir(dir)
	if len(byDir) != 2 {
		t.Errorf("expected 2 modules by dir, got %d: %v", len(byDir), byDir)
	}
}

func TestModuleRegistryLoadModulesFromUntrustedDir(t *testing.T) {
	dir := t.TempDir()

	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)
	mr.WithTrustedDirs([]string{"/etc/mikrolab/modules"})

	err := mr.LoadModulesFromDir(dir)
	if err == nil {
		t.Fatal("expected error for untrusted directory")
	}
}

func TestModuleRegistryChecksumVerification(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.json")

	content := `{
		"path": "/test",
		"type": "list",
		"title": "Test",
		"schema": { "name": { "type": "string" } },
		"actions": { "add": { "name": "add", "parameters": ["name"] } }
	}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create manifest with wrong checksum using proper JSON marshaling
	manifestPath := filepath.Join(dir, "manifest.json")
	manifest := map[string]string{
		filePath: "0000000000000000000000000000000000000000000000000000000000000000",
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatal(err)
	}

	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)
	mr.WithTrustedDirs([]string{dir})

	if err := mr.LoadManifest(manifestPath); err != nil {
		t.Fatalf("unexpected error loading manifest: %v", err)
	}

	_, err = mr.LoadModuleFile(filePath)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestModuleRegistryEventListeners(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	events := make([]ModuleRegistryEvent, 0)
	eventCh := make(chan ModuleRegistryEvent, 10)

	mr.AddListener(func(event ModuleRegistryEvent) {
		eventCh <- event
	})

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	if err := mr.RegisterModule(schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case event := <-eventCh:
		if event.Type != ModuleLoaded {
			t.Errorf("expected ModuleLoaded event, got %v", event.Type)
		}
		if event.Path != "/test" {
			t.Errorf("expected path /test, got %q", event.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Test unload event
	if err := mr.UnloadModule("/test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case event := <-eventCh:
		if event.Type != ModuleUnloaded {
			t.Errorf("expected ModuleUnloaded event, got %v", event.Type)
		}
		if event.Path != "/test" {
			t.Errorf("expected path /test, got %q", event.Path)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for unload event")
	}

	_ = events // used for debugging
}

func TestModuleRegistryGetManager(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	manager := mr.GetManager()
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	// Registration through registry should also be visible via manager
	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}
	mr.RegisterModule(schema)

	s, ok := manager.GetSchema("/test")
	if !ok {
		t.Fatal("expected schema to be visible via manager")
	}
	if s.Title != "Test" {
		t.Errorf("expected title 'Test', got %q", s.Title)
	}
}

func TestModuleRegistryExecuteOperations(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add":    {Name: "add", Parameters: []string{"name"}},
			"remove": {Name: "remove", Parameters: []string{"numbers"}, Validators: []string{"entry_exists"}},
		},
	}
	mr.RegisterModule(schema)

	manager := mr.GetManager()

	// Add entry
	opAdd := NewOperation(OpAdd, "/test")
	opAdd.Properties["name"] = "test-entry"
	if err := manager.ExecuteOperation(opAdd); err != nil {
		t.Fatalf("unexpected error executing add: %v", err)
	}

	entries, err := ct.GetEntries("/test")
	if err != nil {
		t.Fatalf("unexpected error getting entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Verify via manager
	opRemove := NewOperation(OpRemove, "/test")
	opRemove.EntryID = entries[0].ID
	if err := manager.ExecuteOperation(opRemove); err != nil {
		t.Fatalf("unexpected error executing remove: %v", err)
	}

	entries, _ = ct.GetEntries("/test")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after removal, got %d", len(entries))
	}
}

func TestModuleRegistryStartStopWatcher(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	// Need trusted dirs for watcher
	mr.WithTrustedDirs([]string{t.TempDir()})

	err := mr.StartWatcher()
	if err != nil {
		t.Fatalf("unexpected error starting watcher: %v", err)
	}

	// Should not be able to start again
	err = mr.StartWatcher()
	if err == nil {
		t.Fatal("expected error starting watcher twice")
	}

	// Stop watcher
	mr.StopWatcher()

	// Should be able to start again after stop
	mr.WithTrustedDirs([]string{t.TempDir()})
	err = mr.StartWatcher()
	if err != nil {
		t.Fatalf("unexpected error restarting watcher: %v", err)
	}
	mr.StopWatcher()
}

func TestModuleRegistryStartWatcherNoTrustedDirs(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	err := mr.StartWatcher()
	if err == nil {
		t.Fatal("expected error starting watcher without trusted dirs")
	}
}

func TestModuleRegistryVersionedModule(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	schema := &ModuleSchema{
		Path:    "/test",
		Type:    "list",
		Title:   "Test",
		Version: "v1.0.0",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	if err := mr.RegisterModule(schema); err != nil {
		t.Fatalf("unexpected error registering versioned module: %v", err)
	}

	got, ok := mr.GetSchema("/test")
	if !ok {
		t.Fatal("expected to find schema")
	}
	if got.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %q", got.Version)
	}
}

func TestModuleRegistryDuplicateRegistration(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	schema := &ModuleSchema{
		Path:  "/test",
		Type:  "list",
		Title: "Test",
		Schema: map[string]*SchemaProperty{
			"name": {Type: SchemaString},
		},
		Actions: map[string]*SchemaAction{
			"add": {Name: "add", Parameters: []string{"name"}},
		},
	}

	if err := mr.RegisterModule(schema); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mr.RegisterModule(schema); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestModuleRegistryUnloadNonExistent(t *testing.T) {
	ct := NewConfigTree()
	vr := NewValidatorRegistry()
	mr := NewModuleRegistry(ct, vr)

	err := mr.UnloadModule("/nonexistent")
	if err == nil {
		t.Fatal("expected error unloading non-existent module")
	}
}
