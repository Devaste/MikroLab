package config

// OperationType defines the type of a configuration operation.
type OperationType string

const (
	OpAdd     OperationType = "add"
	OpSet     OperationType = "set"
	OpRemove  OperationType = "remove"
	OpPrint   OperationType = "print"
	OpExport  OperationType = "export"
	OpDisable OperationType = "disable"
	OpEnable  OperationType = "enable"
	OpMove    OperationType = "move"
)

// Operation represents a single configuration operation to be applied to the tree.
type Operation struct {
	Type       OperationType          `json:"type"`
	Path       string                 `json:"path"`
	EntryID    string                 `json:"entryId,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Flags      map[string]bool        `json:"flags,omitempty"`
	Numbers    []string               `json:"numbers,omitempty"`
	Where      map[string]interface{} `json:"where,omitempty"`
}

// NewOperation creates a new operation.
func NewOperation(opType OperationType, path string) *Operation {
	return &Operation{
		Type:       opType,
		Path:       path,
		Properties: make(map[string]interface{}),
		Flags:      make(map[string]bool),
	}
}
