package api

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// Compiler caches compiled JSON schemas for runtime validation.
type Compiler struct {
	schemas map[string]*jsonschema.Schema
}

// NewCompiler compiles all API schemas from the api/schemas/ directory.
// It finds the schema directory relative to the source file location.
func NewCompiler() (*Compiler, error) {
	// Determine the schema directory relative to the repository root.
	// We resolve from the source file location or fall back to relative path.
	schemaDir := findSchemaDir()
	if schemaDir == "" {
		return nil, fmt.Errorf("cannot find api/schemas/ directory")
	}

	c := &Compiler{
		schemas: make(map[string]*jsonschema.Schema),
	}

	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7

	schemaFiles := []string{
		"CommandRequest.json",
		"CommandResponse.json",
		"EventMessage.json",
	}

	for _, sf := range schemaFiles {
		path := filepath.Join(schemaDir, sf)
		schema, err := compiler.Compile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to compile schema %s: %w", sf, err)
		}
		c.schemas[sf] = schema
	}

	return c, nil
}

// Get returns the compiled schema for the given filename.
func (c *Compiler) Get(name string) *jsonschema.Schema {
	return c.schemas[name]
}

// findSchemaDir attempts to locate the api/schemas/ directory.
// It tries several strategies to handle both source and build-time contexts.
func findSchemaDir() string {
	// Strategy 1: relative to the working directory (common during development)
	if _, err := os.Stat("api/schemas/CommandRequest.json"); err == nil {
		abs, _ := filepath.Abs("api/schemas")
		return abs
	}

	// Strategy 2: relative to the source file (using runtime.Caller)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// internal/api/validator.go -> up 2 directories = repo root
		base := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
		candidate := filepath.Join(base, "api", "schemas")
		if _, err := os.Stat(filepath.Join(candidate, "CommandRequest.json")); err == nil {
			return candidate
		}
	}

	// Strategy 3: check if we're inside a test directory
	cwd, _ := os.Getwd()
	for _, candidate := range []string{
		filepath.Join(cwd, "..", "..", "api", "schemas"),
		filepath.Join(cwd, "..", "..", "..", "api", "schemas"),
	} {
		if _, err := os.Stat(filepath.Join(candidate, "CommandRequest.json")); err == nil {
			return candidate
		}
	}

	return ""
}
