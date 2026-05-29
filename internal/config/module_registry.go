package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModuleRegistryEvent types for registry events.
type ModuleRegistryEventType string

const (
	ModuleLoaded   ModuleRegistryEventType = "module_loaded"
	ModuleUnloaded ModuleRegistryEventType = "module_unloaded"
	ModuleUpdated  ModuleRegistryEventType = "module_updated"
	ModuleError    ModuleRegistryEventType = "module_error"
)

// ModuleRegistryEvent represents an event emitted by the registry.
type ModuleRegistryEvent struct {
	Type    ModuleRegistryEventType `json:"type"`
	Path    string                  `json:"path"`
	Error   string                  `json:"error,omitempty"`
	Version string                  `json:"version,omitempty"`
	Time    time.Time               `json:"time"`
}

// ModuleRegistryListener is a callback for registry events.
type ModuleRegistryListener func(event ModuleRegistryEvent)

// ModuleRegistry provides a secure, dynamic module registration system.
// It wraps ModuleManager with security checks, schema validation,
// dependency resolution, and optional hot-reload support.
type ModuleRegistry struct {
	mu sync.RWMutex

	manager         *ModuleManager
	schemaValidator *SchemaValidator
	validatorReg    *ValidatorRegistry

	// Trusted directories for module loading
	trustedDirs []string

	// Track which modules were loaded from which files
	moduleFiles map[string]string // path -> source file

	// Hot-reload support
	watcherEnabled bool
	watcherStopCh  chan struct{}
	watchInterval  time.Duration

	// Event listeners
	listeners []ModuleRegistryListener

	// Optional manifest checksums
	manifest map[string]string // source file -> sha256 checksum
}

// NewModuleRegistry creates a new secure module registry.
func NewModuleRegistry(tree *ConfigTree, vr *ValidatorRegistry) *ModuleRegistry {
	return &ModuleRegistry{
		manager:         NewModuleManager(tree, vr),
		schemaValidator: NewSchemaValidator(),
		validatorReg:    vr,
		trustedDirs:     make([]string, 0),
		moduleFiles:     make(map[string]string),
		watchInterval:   5 * time.Second,
		manifest:        make(map[string]string),
	}
}

// WithTrustedDirs configures trusted directories for secure module loading.
func (mr *ModuleRegistry) WithTrustedDirs(dirs []string) *ModuleRegistry {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	mr.trustedDirs = make([]string, len(dirs))
	for i, d := range dirs {
		abs, err := filepath.Abs(d)
		if err == nil {
			mr.trustedDirs[i] = abs
		} else {
			mr.trustedDirs[i] = d
		}
	}
	mr.schemaValidator = mr.schemaValidator.WithTrustedSourceDirs(mr.trustedDirs)
	return mr
}

// WithWatchInterval sets the hot-reload polling interval.
func (mr *ModuleRegistry) WithWatchInterval(d time.Duration) *ModuleRegistry {
	mr.watchInterval = d
	return mr
}

// AddListener registers a listener for registry events.
func (mr *ModuleRegistry) AddListener(listener ModuleRegistryListener) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.listeners = append(mr.listeners, listener)
}

// LoadManifest loads a JSON manifest file containing expected checksums.
// Format: { "module_file.json": "sha256hex", ... }
func (mr *ModuleRegistry) LoadManifest(manifestPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest %s: %w", manifestPath, err)
	}

	var manifest map[string]string
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest %s: %w", manifestPath, err)
	}

	mr.mu.Lock()
	mr.manifest = manifest
	mr.mu.Unlock()

	return nil
}

// RegisterModule registers a module after schema validation and security checks.
func (mr *ModuleRegistry) RegisterModule(schema *ModuleSchema) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// 1. Security check: verify source file is in trusted directory
	if schema.SourceFile != "" {
		securityResult := mr.schemaValidator.ValidatePathSecurity(schema)
		if securityResult.HasErrors() {
			return fmt.Errorf("module registration rejected: %w", securityResult)
		}
	}

	// 2. Checksum verification if manifest has an entry for this source file
	if schema.SourceFile != "" {
		if expectedChecksum, ok := mr.manifest[schema.SourceFile]; ok {
			actualChecksum, err := mr.computeFileChecksum(schema.SourceFile)
			if err != nil {
				return fmt.Errorf("checksum computation failed for %s: %w", schema.SourceFile, err)
			}
			if actualChecksum != expectedChecksum {
				return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
					schema.SourceFile, expectedChecksum, actualChecksum)
			}
			schema.Checksum = actualChecksum
		}
	}

	// 3. Schema validation
	vr := mr.validatorReg
	validationResult := mr.schemaValidator.Validate(schema, vr)
	if validationResult.HasErrors() {
		return fmt.Errorf("schema validation failed: %w", validationResult)
	}

	// 4. Dependency validation — ensure all dependencies are already registered
	for _, dep := range schema.Dependencies {
		if _, exists := mr.manager.Modules[dep.Path]; !exists {
			return fmt.Errorf("unresolved dependency: module %q requires %q which is not registered",
				schema.Path, dep.Path)
		}
	}

	// 5. Check for overlapping path segments with existing modules
	if err := mr.checkPathOverlap(schema); err != nil {
		return err
	}

	// 6. Register via the underlying manager
	if err := mr.manager.RegisterModule(schema); err != nil {
		return err
	}

	// 7. Track source file mapping
	if schema.SourceFile != "" {
		mr.moduleFiles[schema.Path] = schema.SourceFile
	}

	// 8. Emit event
	mr.emitEvent(ModuleRegistryEvent{
		Type:    ModuleLoaded,
		Path:    schema.Path,
		Version: schema.Version,
		Time:    time.Now(),
	})

	return nil
}

// LoadModuleFile loads a single module from a file with full validation.
func (mr *ModuleRegistry) LoadModuleFile(filePath string) (*ModuleSchema, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve path %s: %w", filePath, err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read module file %s: %w", absPath, err)
	}

	schema := &ModuleSchema{}
	if err := json.Unmarshal(data, schema); err != nil {
		return nil, fmt.Errorf("failed to parse module schema %s: %w", absPath, err)
	}

	schema.SourceFile = absPath

	if err := mr.RegisterModule(schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// LoadModulesFromDir loads all JSON modules from a trusted directory.
func (mr *ModuleRegistry) LoadModulesFromDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("cannot resolve directory %s: %w", dir, err)
	}

	// Verify directory is trusted
	trusted := false
	for _, td := range mr.trustedDirs {
		rel, err := filepath.Rel(td, absDir)
		if err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			trusted = true
			break
		}
		// Also check if the dir itself is a trusted dir
		if absDir == td {
			trusted = true
			break
		}
	}

	if !trusted {
		return fmt.Errorf("directory %s is not in the trusted directories list", absDir)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", absDir, err)
	}

	// Sort entries for deterministic loading order (respects dependencies)
	fileNames := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		fileNames = append(fileNames, entry.Name())
	}
	sort.Strings(fileNames)

	for _, fileName := range fileNames {
		filePath := filepath.Join(absDir, fileName)
		if _, err := mr.LoadModuleFile(filePath); err != nil {
			return fmt.Errorf("failed to load module %s: %w", fileName, err)
		}
	}

	return nil
}

// UnloadModule removes a module from the registry by its path.
func (mr *ModuleRegistry) UnloadModule(path string) error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Check no other module depends on this one
	for _, other := range mr.manager.Modules {
		if other.Path == path {
			continue
		}
		for _, dep := range other.Dependencies {
			if dep.Path == path {
				return fmt.Errorf("cannot unload %q: module %q depends on it", path, other.Path)
			}
		}
	}

	schema, exists := mr.manager.Modules[path]
	if !exists {
		return fmt.Errorf("module %q is not registered", path)
	}

	delete(mr.manager.Modules, path)
	delete(mr.moduleFiles, path)

	mr.emitEvent(ModuleRegistryEvent{
		Type:    ModuleUnloaded,
		Path:    path,
		Version: schema.Version,
		Time:    time.Now(),
	})

	return nil
}

// GetManager returns the underlying ModuleManager for operation execution.
func (mr *ModuleRegistry) GetManager() *ModuleManager {
	return mr.manager
}

// GetSchema returns a module schema by path.
func (mr *ModuleRegistry) GetSchema(path string) (*ModuleSchema, bool) {
	return mr.manager.GetSchema(path)
}

// ListModules returns all registered module paths.
func (mr *ModuleRegistry) ListModules() []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	paths := make([]string, 0, len(mr.manager.Modules))
	for p := range mr.manager.Modules {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// ListModulesByDir returns module paths that were loaded from a specific directory.
func (mr *ModuleRegistry) ListModulesByDir(dir string) []string {
	mr.mu.RLock()
	defer mr.mu.RUnlock()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	paths := make([]string, 0)
	for path, srcFile := range mr.moduleFiles {
		rel, err := filepath.Rel(absDir, srcFile)
		if err == nil && !strings.HasPrefix(rel, "..") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

// StartWatcher begins polling trusted directories for module changes.
// Watches for file modifications and reloads changed modules.
func (mr *ModuleRegistry) StartWatcher() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if len(mr.trustedDirs) == 0 {
		return fmt.Errorf("no trusted directories configured for watching")
	}
	if mr.watcherEnabled {
		return fmt.Errorf("watcher is already running")
	}

	mr.watcherEnabled = true
	mr.watcherStopCh = make(chan struct{})

	// Collect last modified times for watched files
	lastModTimes := make(map[string]time.Time)
	for _, schema := range mr.manager.Modules {
		if schema.SourceFile != "" {
			if fi, err := os.Stat(schema.SourceFile); err == nil {
				lastModTimes[schema.SourceFile] = fi.ModTime()
			}
		}
	}

	go mr.watchLoop(lastModTimes)
	return nil
}

// StopWatcher stops the hot-reload polling loop.
func (mr *ModuleRegistry) StopWatcher() {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	if mr.watcherEnabled {
		close(mr.watcherStopCh)
		mr.watcherEnabled = false
	}
}

// computeFileChecksum computes SHA-256 of a file.
func (mr *ModuleRegistry) computeFileChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// checkPathOverlap ensures no two modules claim overlapping tree paths.
func (mr *ModuleRegistry) checkPathOverlap(schema *ModuleSchema) error {
	newPath := schema.Path

	for existingPath := range mr.manager.Modules {
		// Check if new path is a parent/child of existing path
		if strings.HasPrefix(existingPath, newPath+"/") {
			return fmt.Errorf("path overlap: new module %q is a parent of existing module %q",
				newPath, existingPath)
		}
		if strings.HasPrefix(newPath, existingPath+"/") {
			return fmt.Errorf("path overlap: new module %q is a child of existing module %q",
				newPath, existingPath)
		}
	}

	return nil
}

// emitEvent sends an event to all registered listeners.
func (mr *ModuleRegistry) emitEvent(event ModuleRegistryEvent) {
	for _, listener := range mr.listeners {
		listener(event)
	}
}

// watchLoop polls files for changes and reloads modified modules.
func (mr *ModuleRegistry) watchLoop(lastModTimes map[string]time.Time) {
	ticker := time.NewTicker(mr.watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mr.checkForChanges(lastModTimes)
		case <-mr.watcherStopCh:
			return
		}
	}
}

// checkForChanges checks all loaded module files for modifications.
func (mr *ModuleRegistry) checkForChanges(lastModTimes map[string]time.Time) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	for path, schema := range mr.manager.Modules {
		if schema.SourceFile == "" {
			continue
		}

		fi, err := os.Stat(schema.SourceFile)
		if err != nil {
			mr.emitEvent(ModuleRegistryEvent{
				Type:  ModuleError,
				Path:  path,
				Error: fmt.Sprintf("cannot stat source file: %v", err),
				Time:  time.Now(),
			})
			continue
		}

		lastMod, exists := lastModTimes[schema.SourceFile]
		if !exists || fi.ModTime().After(lastMod) {
			// File modified — attempt reload
			lastModTimes[schema.SourceFile] = fi.ModTime()

			// Read fresh data
			data, err := os.ReadFile(schema.SourceFile)
			if err != nil {
				mr.emitEvent(ModuleRegistryEvent{
					Type:  ModuleError,
					Path:  path,
					Error: fmt.Sprintf("cannot read updated file: %v", err),
					Time:  time.Now(),
				})
				continue
			}

			newSchema := &ModuleSchema{}
			if err := json.Unmarshal(data, newSchema); err != nil {
				mr.emitEvent(ModuleRegistryEvent{
					Type:  ModuleError,
					Path:  path,
					Error: fmt.Sprintf("cannot parse updated schema: %v", err),
					Time:  time.Now(),
				})
				continue
			}
			newSchema.SourceFile = schema.SourceFile

			// Validate the updated schema
			vr := mr.validatorReg
			validationResult := mr.schemaValidator.Validate(newSchema, vr)
			if validationResult.HasErrors() {
				mr.emitEvent(ModuleRegistryEvent{
					Type:  ModuleError,
					Path:  path,
					Error: fmt.Sprintf("updated schema validation failed: %v", validationResult),
					Time:  time.Now(),
				})
				continue
			}

			// Replace schema in registry
			mr.manager.Modules[path] = newSchema

			// Update tree node schema reference
			if node, err := mr.manager.Tree.Navigate(path); err == nil {
				node.Schema = newSchema
			}

			mr.emitEvent(ModuleRegistryEvent{
				Type:    ModuleUpdated,
				Path:    path,
				Version: newSchema.Version,
				Time:    time.Now(),
			})
		}
	}
}
