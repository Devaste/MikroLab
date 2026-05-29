package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ModuleManager manages all loaded modules.
type ModuleManager struct {
	Modules           map[string]*ModuleSchema
	Tree              *ConfigTree
	ValidatorRegistry *ValidatorRegistry
}

// NewModuleManager creates a new module manager.
func NewModuleManager(tree *ConfigTree, vr *ValidatorRegistry) *ModuleManager {
	return &ModuleManager{
		Modules:           make(map[string]*ModuleSchema),
		Tree:              tree,
		ValidatorRegistry: vr,
	}
}

// LoadModule loads a module schema from a JSON file and registers it.
func (mm *ModuleManager) LoadModule(filePath string) (*ModuleSchema, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module file %s: %w", filePath, err)
	}

	schema := &ModuleSchema{}
	if err := json.Unmarshal(data, schema); err != nil {
		return nil, fmt.Errorf("failed to parse module schema %s: %w", filePath, err)
	}

	return schema, mm.RegisterModule(schema)
}

// RegisterModule registers a module schema and creates its tree path.
func (mm *ModuleManager) RegisterModule(schema *ModuleSchema) error {
	if _, exists := mm.Modules[schema.Path]; exists {
		return fmt.Errorf("module already registered at path %s", schema.Path)
	}

	mm.Modules[schema.Path] = schema

	// Ensure tree path exists
	nodeType := NodeTypeDirectory
	if schema.Type == "list" {
		nodeType = NodeTypeList
	}

	node, err := mm.Tree.EnsurePath(schema.Path, nodeType, schema.Title)
	if err != nil {
		return fmt.Errorf("failed to create tree path %s: %w", schema.Path, err)
	}

	node.Schema = schema
	return nil
}

// LoadModulesFromDir loads all JSON module schemas from a directory.
func (mm *ModuleManager) LoadModulesFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read modules directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		if _, err := mm.LoadModule(filePath); err != nil {
			return fmt.Errorf("failed to load module %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// GetSchema returns the schema for a given path.
func (mm *ModuleManager) GetSchema(path string) (*ModuleSchema, bool) {
	schema, ok := mm.Modules[path]
	return schema, ok
}

// ExecuteOperation executes an operation with validation.
func (mm *ModuleManager) ExecuteOperation(op *Operation) error {
	schema, ok := mm.Modules[op.Path]
	if !ok {
		return fmt.Errorf("no module registered for path %s", op.Path)
	}

	action, ok := schema.GetAction(string(op.Type))
	if !ok {
		return fmt.Errorf("action %q not supported by module %s", op.Type, op.Path)
	}

	// Validate
	result := mm.ValidatorRegistry.Validate(action.Validators, op, mm.Tree)
	if result.HasErrors() {
		return fmt.Errorf("validation failed: %w", result)
	}

	// Execute based on operation type
	switch op.Type {
	case OpAdd:
		return mm.executeAdd(schema, op)
	case OpSet:
		return mm.executeSet(schema, op)
	case OpRemove:
		return mm.executeRemove(schema, op)
	case OpDisable:
		return mm.executeToggleDisable(schema, op, true)
	case OpEnable:
		return mm.executeToggleDisable(schema, op, false)
	default:
		return fmt.Errorf("operation %q not yet implemented", op.Type)
	}
}

func (mm *ModuleManager) executeAdd(schema *ModuleSchema, op *Operation) error {
	entry := NewEntry("", 0)

	// Apply defaults
	for name, val := range schema.Defaults {
		entry.Properties[name] = &PropertyValue{
			Name:  name,
			Value: val,
		}
	}

	// Set provided properties
	for name, val := range op.Properties {
		if _, ok := entry.Properties[name]; !ok {
			// Look up in schema
			propDef, found := schema.GetProperty(name)
			if !found {
				entry.Properties[name] = &PropertyValue{Name: name, Value: val}
			} else {
				entry.Properties[name] = &PropertyValue{
					Name:     name,
					Type:     string(propDef.Type),
					Value:    val,
					ReadOnly: propDef.ReadOnly,
					Required: propDef.Required,
				}
			}
		} else {
			entry.Properties[name].Value = val
		}
	}

	// Apply flags from action
	if action, ok := schema.GetAction("add"); ok {
		for _, flagName := range action.FlagsSet {
			switch flagName {
			case "disabled":
				entry.Disabled = true
			}
		}
	}

	return mm.Tree.AddEntry(op.Path, entry)
}

func (mm *ModuleManager) executeSet(schema *ModuleSchema, op *Operation) error {
	return mm.Tree.SetEntry(op.Path, op.EntryID, op.Properties)
}

func (mm *ModuleManager) executeRemove(schema *ModuleSchema, op *Operation) error {
	return mm.Tree.RemoveEntry(op.Path, op.EntryID)
}

func (mm *ModuleManager) executeToggleDisable(schema *ModuleSchema, op *Operation, disable bool) error {
	node, err := mm.Tree.Navigate(op.Path)
	if err != nil {
		return err
	}
	if node.Type != NodeTypeList {
		return fmt.Errorf("node %s is not a list", op.Path)
	}

	node.mu.Lock()
	defer node.mu.Unlock()

	var target *Entry
	for _, e := range node.Entries {
		if e.ID == op.EntryID {
			target = e
			break
		}
	}
	if target == nil {
		return fmt.Errorf("entry %s not found in %s", op.EntryID, op.Path)
	}

	target.Disabled = disable

	mm.Tree.EventBus.Emit(Event{
		Path:  op.Path,
		Type:  EventUpdate,
		Entry: target,
	})

	return nil
}
