package config

import (
	"fmt"
)

// ValidateOperation performs comprehensive validation of an operation
// before it is executed. This includes:
//   - Path validation (module must exist)
//   - Action validation (action must be supported by the module)
//   - Property type coercion and sanitization
//   - Entry ID format validation
//   - Numbers field validation
//   - Flags validation
//   - Where clause validation
//
// Returns a sanitized copy of the operation with coerced values, or an error.
func ValidateOperation(op *Operation, mm *ModuleManager) (*Operation, error) {
	// Validate path exists
	schema, ok := mm.Modules[op.Path]
	if !ok {
		return nil, fmt.Errorf("no module registered for path %s", op.Path)
	}

	// Validate action is supported
	action, ok := schema.GetAction(string(op.Type))
	if !ok {
		return nil, fmt.Errorf("action %q not supported by module %s", op.Type, op.Path)
	}

	// Clone and sanitize the operation
	validated := NewOperation(op.Type, op.Path)
	validated.EntryID = op.EntryID

	// Validate EntryID if present
	if err := ValidateEntryID(op.EntryID); err != nil {
		return nil, err
	}

	// Coerce and validate properties against the schema
	for name, rawVal := range op.Properties {
		propDef, exists := schema.GetProperty(name)
		if !exists {
			// If the property isn't in the schema, check if it's a known
			// built-in parameter (like "numbers", "where", etc.)
			if isBuiltInParam(name) {
				validated.Properties[name] = rawVal
				continue
			}
			return nil, fmt.Errorf("property %q is not defined in module %s", name, op.Path)
		}

		// Coerce and sanitize the value
		coercedVal, err := CoercePropertyValue(rawVal, propDef.Type)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}
		validated.Properties[name] = coercedVal
	}

	// Validate numbers field
	if err := ValidateNumbers(op.Numbers); err != nil {
		return nil, fmt.Errorf("numbers: %w", err)
	}
	validated.Numbers = make([]string, len(op.Numbers))
	copy(validated.Numbers, op.Numbers)

	// Validate where clause
	if len(op.Where) > 0 {
		if err := ValidateWhere(op.Where, schema); err != nil {
			return nil, fmt.Errorf("where: %w", err)
		}
		validated.Where = make(map[string]interface{})
		for k, v := range op.Where {
			validated.Where[k] = v
		}
	}

	// Validate flags
	if len(op.Flags) > 0 {
		if err := ValidateFlags(op.Flags, schema); err != nil {
			return nil, err
		}
		validated.Flags = make(map[string]bool)
		for k, v := range op.Flags {
			validated.Flags[k] = v
		}
	}

	// Validate required properties based on action parameters
	// Only enforce required properties for "add" operations;
	// "set" and other operations may modify only a subset of properties.
	if op.Type == OpAdd {
		for _, param := range action.Parameters {
			if param == "numbers" || param == "where" || param == "detail" || param == "follow" || param == "compact" {
				continue // built-in parameters, skip required check
			}
			propDef, exists := schema.GetProperty(param)
			if exists && propDef.Required {
				if _, hasVal := validated.Properties[param]; !hasVal {
					// Check if default exists
					if _, hasDefault := schema.Defaults[param]; !hasDefault {
						return nil, fmt.Errorf("required property %q is missing for action %q", param, op.Type)
					}
				}
			}
		}
	}

	return validated, nil
}

// isBuiltInParam returns true if the parameter name is a RouterOS built-in.
func isBuiltInParam(name string) bool {
	switch name {
	case "numbers", "where", "detail", "follow", "compact", "from", "to", "place-before", "copy-from":
		return true
	default:
		return false
	}
}
