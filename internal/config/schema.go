package config

// SchemaPropertyType defines the type of a schema property.
type SchemaPropertyType string

const (
	SchemaString      SchemaPropertyType = "string"
	SchemaInteger     SchemaPropertyType = "integer"
	SchemaBoolean     SchemaPropertyType = "bool"
	SchemaIPAddr      SchemaPropertyType = "ipAddr"
	SchemaMACAddr     SchemaPropertyType = "macAddr"
	SchemaIPPrefix    SchemaPropertyType = "ipPrefix"
	SchemaEnum        SchemaPropertyType = "enum"
	SchemaInterface   SchemaPropertyType = "interface_enum"
	SchemaComposite   SchemaPropertyType = "composite_arg"
	SchemaCompositeIP SchemaPropertyType = "composite_ip"
)

// SchemaFlag describes a flag definition in a module schema.
type SchemaFlag struct {
	Letter      string `json:"letter"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SchemaProperty describes a property definition in a module schema.
type SchemaProperty struct {
	Name          string                     `json:"-"`
	Type          SchemaPropertyType         `json:"type"`
	Separator     string                     `json:"separator,omitempty"`
	Required      bool                       `json:"required"`
	Description   string                     `json:"description,omitempty"`
	ReadOnly      bool                       `json:"readOnly,omitempty"`
	ComputedFrom  string                     `json:"computedFrom,omitempty"`
	DynamicValues bool                       `json:"dynamicValues,omitempty"`
	Ref           string                     `json:"ref,omitempty"`
	Default       interface{}                `json:"default,omitempty"`
	Components    map[string]*SchemaProperty `json:"components,omitempty"`
}

// SchemaAction describes an action definition in a module schema.
type SchemaAction struct {
	Name        string   `json:"name"`
	Parameters  []string `json:"parameters"`
	Validators  []string `json:"validators"`
	FlagsSet    []string `json:"flags_set,omitempty"`
	Description string   `json:"description"`
}

// ModuleSchema defines the schema for a module.
type ModuleSchema struct {
	Path        string                     `json:"path"`
	Type        string                     `json:"type"`
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Flags       []SchemaFlag               `json:"flags"`
	Schema      map[string]*SchemaProperty `json:"schema"`
	Actions     map[string]*SchemaAction   `json:"actions"`
	Defaults    map[string]interface{}     `json:"defaults"`
	Constraints map[string]string          `json:"constraints"`
}

// GetAction returns an action definition by name.
func (ms *ModuleSchema) GetAction(name string) (*SchemaAction, bool) {
	action, ok := ms.Actions[name]
	return action, ok
}

// GetProperty returns a schema property definition by name.
func (ms *ModuleSchema) GetProperty(name string) (*SchemaProperty, bool) {
	prop, ok := ms.Schema[name]
	return prop, ok
}
