package core

// Command represents a non-persistent command node in the configuration tree.
// Commands do not store state; they execute an action when invoked and return
// a result. Examples include /ping, /tool/ping, /tool/traceroute.
type Command interface {
	Node

	// Execute runs the command with the given arguments and returns a result.
	// The args map contains string-keyed parameter values.
	Execute(args map[string]interface{}) (interface{}, error)
}
