package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SchemaValidationError represents a single validation error on a module schema.
type SchemaValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (sve *SchemaValidationError) Error() string {
	return fmt.Sprintf("schema validation error at %s: %s", sve.Field, sve.Message)
}

// SchemaValidationResult holds all schema validation errors.
type SchemaValidationResult struct {
	Errors []SchemaValidationError `json:"errors"`
}

func (svr *SchemaValidationResult) HasErrors() bool {
	return len(svr.Errors) > 0
}

func (svr *SchemaValidationResult) Error() string {
	if !svr.HasErrors() {
		return ""
	}
	msgs := make([]string, len(svr.Errors))
	for i, e := range svr.Errors {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// SchemaValidator validates module schemas for structural integrity.
type SchemaValidator struct {
	trustedSourceDirs []string
}

// NewSchemaValidator creates a new schema validator.
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

// WithTrustedSourceDirs sets the trusted source directories for path validation.
func (sv *SchemaValidator) WithTrustedSourceDirs(dirs []string) *SchemaValidator {
	sv.trustedSourceDirs = dirs
	return sv
}

// Validate checks a module schema for structural integrity.
// It validates:
//   - Required fields exist
//   - Type is a known value
//   - Path format is valid
//   - Property types are known
//   - Action parameters reference valid schema properties
//   - Validator names reference known validators (if ValidatorRegistry provided)
//   - Flag letter/name conventions
//   - String field lengths and content sanitization
func (sv *SchemaValidator) Validate(schema *ModuleSchema, vr *ValidatorRegistry) *SchemaValidationResult {
	result := &SchemaValidationResult{}

	// 1. Required fields
	sv.validateRequiredFields(schema, result)
	if result.HasErrors() {
		return result // can't continue if basics are missing
	}

	// 2. Path validation
	sv.validatePath(schema, result)

	// 3. Type validation
	sv.validateType(schema, result)

	// 4. Flag validation
	sv.validateFlags(schema, result)

	// 5. Schema property validation
	sv.validateProperties(schema, result)

	// 6. Action validation
	sv.validateActions(schema, vr, result)

	// 7. Dependency validation
	sv.validateDependencies(schema, result)

	// 8. Version validation
	sv.validateVersion(schema, result)

	// 9. String field sanitization validation
	sv.validateStringContent(schema, result)

	return result
}

// ValidatePathSecurity checks that a module's source file is within a trusted directory.
func (sv *SchemaValidator) ValidatePathSecurity(schema *ModuleSchema) *SchemaValidationResult {
	result := &SchemaValidationResult{}
	if schema.SourceFile == "" {
		return result // no source file to validate
	}

	absSource, err := filepath.Abs(schema.SourceFile)
	if err != nil {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "SourceFile",
			Message: fmt.Sprintf("cannot resolve source file path: %v", err),
		})
		return result
	}

	if len(sv.trustedSourceDirs) == 0 {
		return result // no trusted dirs configured, skip security check
	}

	allowed := false
	for _, trustedDir := range sv.trustedSourceDirs {
		absTrusted, err := filepath.Abs(trustedDir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absTrusted, absSource)
		if err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			allowed = true
			break
		}
	}

	if !allowed {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "SourceFile",
			Message: fmt.Sprintf("source file %s is not in a trusted directory", schema.SourceFile),
		})
	}

	return result
}

func (sv *SchemaValidator) validateRequiredFields(schema *ModuleSchema, result *SchemaValidationResult) {
	if schema.Path == "" {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "path",
			Message: "path is required",
		})
	}
	if schema.Type == "" {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "type",
			Message: "type is required",
		})
	}
	if schema.Title == "" {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "title",
			Message: "title is required",
		})
	}
	if len(schema.Schema) == 0 {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "schema",
			Message: "at least one schema property is required",
		})
	}
	if len(schema.Actions) == 0 {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "actions",
			Message: "at least one action is required",
		})
	}
}

func (sv *SchemaValidator) validatePath(schema *ModuleSchema, result *SchemaValidationResult) {
	path := schema.Path
	if path == "" {
		return // already handled in required fields
	}

	// Must start with /
	if !strings.HasPrefix(path, "/") {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "path",
			Message: fmt.Sprintf("path %q must start with /", path),
		})
	}

	// Must not have trailing slash (unless root)
	if path != "/" && strings.HasSuffix(path, "/") {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "path",
			Message: fmt.Sprintf("path %q must not have trailing slash", path),
		})
	}

	// Each segment must be a valid identifier
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if strings.ContainsAny(seg, " .\\<>:\"|?*") {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   "path",
				Message: fmt.Sprintf("path segment %q contains invalid characters", seg),
			})
		}
	}
}

func (sv *SchemaValidator) validateType(schema *ModuleSchema, result *SchemaValidationResult) {
	switch schema.Type {
	case "list", "directory":
		// valid
	default:
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "type",
			Message: fmt.Sprintf("unknown module type %q; must be \"list\" or \"directory\"", schema.Type),
		})
	}
}

func (sv *SchemaValidator) validateFlags(schema *ModuleSchema, result *SchemaValidationResult) {
	seenLetters := make(map[string]bool)
	seenNames := make(map[string]bool)

	for i, flag := range schema.Flags {
		// Letter must be a single uppercase letter
		if len(flag.Letter) != 1 || flag.Letter[0] < 'A' || flag.Letter[0] > 'Z' {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("flags[%d].letter", i),
				Message: fmt.Sprintf("flag letter must be a single uppercase letter, got %q", flag.Letter),
			})
		}
		if seenLetters[flag.Letter] {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("flags[%d].letter", i),
				Message: fmt.Sprintf("duplicate flag letter %q", flag.Letter),
			})
		}
		seenLetters[flag.Letter] = true

		// Name must be non-empty and lowercase
		if flag.Name == "" {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("flags[%d].name", i),
				Message: "flag name is required",
			})
		}
		if seenNames[flag.Name] {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("flags[%d].name", i),
				Message: fmt.Sprintf("duplicate flag name %q", flag.Name),
			})
		}
		seenNames[flag.Name] = true
	}
}

func (sv *SchemaValidator) validateProperties(schema *ModuleSchema, result *SchemaValidationResult) {
	knownTypes := map[SchemaPropertyType]bool{
		SchemaString:      true,
		SchemaInteger:     true,
		SchemaBoolean:     true,
		SchemaIPAddr:      true,
		SchemaMACAddr:     true,
		SchemaIPPrefix:    true,
		SchemaEnum:        true,
		SchemaInterface:   true,
		SchemaComposite:   true,
		SchemaCompositeIP: true,
	}

	for name, prop := range schema.Schema {
		// Property name must not be empty
		if name == "" {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   "schema",
				Message: "property name must not be empty",
			})
			continue
		}

		// Type must be known
		if !knownTypes[prop.Type] {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("schema.%s.type", name),
				Message: fmt.Sprintf("unknown property type %q", prop.Type),
			})
		}

		// If composite type, must have components
		if prop.Type == SchemaComposite || prop.Type == SchemaCompositeIP {
			if len(prop.Components) == 0 {
				result.Errors = append(result.Errors, SchemaValidationError{
					Field:   fmt.Sprintf("schema.%s.components", name),
					Message: fmt.Sprintf("composite property %q must have components defined", name),
				})
			}
		}

		// If dynamicValues is true, ref must be set
		if prop.DynamicValues && prop.Ref == "" {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("schema.%s.ref", name),
				Message: fmt.Sprintf("property %q has dynamicValues=true but no ref", name),
			})
		}
	}
}

func (sv *SchemaValidator) validateActions(schema *ModuleSchema, vr *ValidatorRegistry, result *SchemaValidationResult) {
	knownOpTypes := map[string]bool{
		"add": true, "set": true, "remove": true,
		"print": true, "export": true,
		"disable": true, "enable": true, "move": true,
	}

	for actionName, action := range schema.Actions {
		// Validate parameters reference real schema properties
		for _, param := range action.Parameters {
			if param == "numbers" || param == "where" || param == "detail" || param == "follow" || param == "compact" {
				continue // built-in parameters, skip
			}
			if _, exists := schema.Schema[param]; !exists {
				result.Errors = append(result.Errors, SchemaValidationError{
					Field:   fmt.Sprintf("actions.%s.parameters", actionName),
					Message: fmt.Sprintf("parameter %q references non-existent schema property", param),
				})
			}
		}

		// Validate validators reference known validators
		for _, vName := range action.Validators {
			if vr != nil {
				if _, exists := vr.Get(vName); !exists {
					result.Errors = append(result.Errors, SchemaValidationError{
						Field:   fmt.Sprintf("actions.%s.validators", actionName),
						Message: fmt.Sprintf("validator %q is not registered", vName),
					})
				}
			}
		}

		// Validate flags_set reference real flag names
		for _, flagName := range action.FlagsSet {
			found := false
			for _, f := range schema.Flags {
				if f.Name == flagName {
					found = true
					break
				}
			}
			if !found {
				result.Errors = append(result.Errors, SchemaValidationError{
					Field:   fmt.Sprintf("actions.%s.flags_set", actionName),
					Message: fmt.Sprintf("flag %q is not defined in module flags", flagName),
				})
			}
		}

		// Check if action name is a known operation type (warning, not error)
		if !knownOpTypes[actionName] && actionName != "get" {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("actions.%s", actionName),
				Message: fmt.Sprintf("action name %q is not a standard operation type", actionName),
			})
		}
	}
}

func (sv *SchemaValidator) validateDependencies(schema *ModuleSchema, result *SchemaValidationResult) {
	seenPaths := make(map[string]bool)
	for i, dep := range schema.Dependencies {
		if dep.Path == "" {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("dependencies[%d].path", i),
				Message: "dependency path is required",
			})
			continue
		}
		if !strings.HasPrefix(dep.Path, "/") {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("dependencies[%d].path", i),
				Message: fmt.Sprintf("dependency path %q must start with /", dep.Path),
			})
		}
		if seenPaths[dep.Path] {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("dependencies[%d].path", i),
				Message: fmt.Sprintf("duplicate dependency path %q", dep.Path),
			})
		}
		seenPaths[dep.Path] = true
	}
}

func (sv *SchemaValidator) validateVersion(schema *ModuleSchema, result *SchemaValidationResult) {
	if schema.Version == "" {
		return // version is optional
	}

	// Simple semantic version check: must start with v followed by numbers and dots
	if len(schema.Version) < 2 || schema.Version[0] != 'v' {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "version",
			Message: fmt.Sprintf("version %q must start with 'v' (e.g., v1.0.0)", schema.Version),
		})
	}
}

// validateStringContent checks that schema string fields contain safe content.
// This helps prevent injection through schema metadata fields.
func (sv *SchemaValidator) validateStringContent(schema *ModuleSchema, result *SchemaValidationResult) {
	// Validate Title
	if len(schema.Title) > MaxSchemaTitleLen {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "title",
			Message: fmt.Sprintf("title exceeds maximum length of %d characters", MaxSchemaTitleLen),
		})
	}

	// Validate Description
	if len(schema.Description) > MaxDescriptionLen {
		result.Errors = append(result.Errors, SchemaValidationError{
			Field:   "description",
			Message: fmt.Sprintf("description exceeds maximum length of %d characters", MaxDescriptionLen),
		})
	}

	// Validate path segment lengths
	segments := strings.Split(strings.Trim(schema.Path, "/"), "/")
	for i, seg := range segments {
		if len(seg) > MaxPathSegmentLen {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("path.segments[%d]", i),
				Message: fmt.Sprintf("path segment %q exceeds maximum length of %d", seg, MaxPathSegmentLen),
			})
		}
	}

	// Validate default values match declared types
	for name, val := range schema.Defaults {
		prop, exists := schema.GetProperty(name)
		if !exists {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("defaults.%s", name),
				Message: fmt.Sprintf("default value references non-existent property %q", name),
			})
			continue
		}
		// Try to coerce the default value to verify it's valid for the declared type
		if _, err := CoercePropertyValue(val, prop.Type); err != nil {
			result.Errors = append(result.Errors, SchemaValidationError{
				Field:   fmt.Sprintf("defaults.%s", name),
				Message: fmt.Sprintf("default value %v does not match property type %q: %v", val, prop.Type, err),
			})
		}
	}
}
