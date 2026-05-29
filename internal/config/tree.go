package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// NodeType represents the type of a configuration node.
type NodeType string

const (
	NodeTypeDirectory NodeType = "directory"
	NodeTypeList      NodeType = "list"
)

// Node represents a node in the configuration tree.
type Node struct {
	mu         sync.RWMutex
	Path       string           `json:"path"`
	Type       NodeType         `json:"type"`
	Title      string           `json:"title"`
	Children   map[string]*Node `json:"children,omitempty"`
	Entries    []*Entry         `json:"entries,omitempty"`
	Schema     *ModuleSchema    `json:"-"`
	EntryIndex int              `json:"-"`
	Parent     *Node            `json:"-"`
}

// NewNode creates a new node.
func NewNode(path string, nodeType NodeType, title string) *Node {
	return &Node{
		Path:     path,
		Type:     nodeType,
		Title:    title,
		Children: make(map[string]*Node),
		Entries:  make([]*Entry, 0),
	}
}

// ConfigTree is the root configuration tree.
type ConfigTree struct {
	mu       sync.RWMutex
	Root     *Node
	EventBus *EventBus
}

// NewConfigTree creates a new configuration tree.
func NewConfigTree() *ConfigTree {
	return &ConfigTree{
		Root:     NewNode("/", NodeTypeDirectory, "root"),
		EventBus: NewEventBus(),
	}
}

// Navigate traverses the tree following the given path segments.
func (ct *ConfigTree) Navigate(path string) (*Node, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	// Normalize path
	path = strings.Trim(path, "/")
	if path == "" {
		return ct.Root, nil
	}

	segments := strings.Split(path, "/")
	current := ct.Root

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		child, ok := current.Children[seg]
		if !ok {
			return nil, fmt.Errorf("path /%s not found", path)
		}
		current = child
	}

	return current, nil
}

// EnsurePath creates all nodes along the path and returns the final node.
func (ct *ConfigTree) EnsurePath(path string, nodeType NodeType, title string) (*Node, error) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	path = strings.Trim(path, "/")
	if path == "" {
		return ct.Root, nil
	}

	segments := strings.Split(path, "/")
	current := ct.Root

	for i, seg := range segments {
		if seg == "" {
			continue
		}
		child, ok := current.Children[seg]
		if !ok {
			segPath := "/" + strings.Join(segments[:i+1], "/")
			isLast := (i == len(segments)-1)
			t := NodeTypeDirectory
			if isLast {
				t = nodeType
			}
			nodeTitle := seg
			if isLast && title != "" {
				nodeTitle = title
			}
			child = NewNode(segPath, t, nodeTitle)
			child.Parent = current
			current.Children[seg] = child
		} else if i == len(segments)-1 {
			// Last segment already exists, update type and title if needed
			if child.Type == NodeTypeDirectory && nodeType == NodeTypeList {
				child.Type = nodeType
			}
			if title != "" && child.Title == seg {
				child.Title = title
			}
		}
		current = child
	}

	return current, nil
}

// AddEntry adds a new entry to a list node.
func (ct *ConfigTree) AddEntry(listPath string, entry *Entry) error {
	node, err := ct.Navigate(listPath)
	if err != nil {
		return fmt.Errorf("cannot add entry: %w", err)
	}
	if node.Type != NodeTypeList {
		return fmt.Errorf("node %s is not a list", listPath)
	}

	node.mu.Lock()
	entry.Index = node.EntryIndex
	node.EntryIndex++
	entry.ID = uuid.New().String()
	node.Entries = append(node.Entries, entry)
	node.mu.Unlock()

	ct.EventBus.Emit(Event{
		Path:  listPath,
		Type:  EventAdd,
		Entry: entry,
	})

	return nil
}

// RemoveEntry removes an entry from a list node by ID.
func (ct *ConfigTree) RemoveEntry(listPath string, entryID string) error {
	node, err := ct.Navigate(listPath)
	if err != nil {
		return fmt.Errorf("cannot remove entry: %w", err)
	}
	if node.Type != NodeTypeList {
		return fmt.Errorf("node %s is not a list", listPath)
	}

	node.mu.Lock()
	found := false
	for i, e := range node.Entries {
		if e.ID == entryID {
			if e.Dynamic {
				node.mu.Unlock()
				return fmt.Errorf("cannot remove dynamic entry")
			}
			node.Entries = append(node.Entries[:i], node.Entries[i+1:]...)
			found = true
			break
		}
	}
	node.mu.Unlock()

	if !found {
		return fmt.Errorf("entry %s not found in %s", entryID, listPath)
	}

	ct.EventBus.Emit(Event{
		Path: listPath,
		Type: EventRemove,
	})

	return nil
}

// GetEntryByID retrieves an entry by ID from a list node.
func (ct *ConfigTree) GetEntryByID(listPath string, entryID string) (*Entry, error) {
	node, err := ct.Navigate(listPath)
	if err != nil {
		return nil, err
	}
	if node.Type != NodeTypeList {
		return nil, fmt.Errorf("node %s is not a list", listPath)
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	for _, e := range node.Entries {
		if e.ID == entryID {
			return e.Clone(), nil
		}
	}

	return nil, fmt.Errorf("entry %s not found in %s", entryID, listPath)
}

// GetEntries returns all entries from a list node.
func (ct *ConfigTree) GetEntries(listPath string) ([]*Entry, error) {
	node, err := ct.Navigate(listPath)
	if err != nil {
		return nil, err
	}
	if node.Type != NodeTypeList {
		return nil, fmt.Errorf("node %s is not a list", listPath)
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	entries := make([]*Entry, len(node.Entries))
	for i, e := range node.Entries {
		entries[i] = e.Clone()
	}
	return entries, nil
}

// SetEntry updates an entry's properties.
func (ct *ConfigTree) SetEntry(listPath string, entryID string, props map[string]interface{}) error {
	node, err := ct.Navigate(listPath)
	if err != nil {
		return fmt.Errorf("cannot set entry: %w", err)
	}
	if node.Type != NodeTypeList {
		return fmt.Errorf("node %s is not a list", listPath)
	}

	node.mu.Lock()
	var target *Entry
	for _, e := range node.Entries {
		if e.ID == entryID {
			target = e
			break
		}
	}
	if target == nil {
		node.mu.Unlock()
		return fmt.Errorf("entry %s not found in %s", entryID, listPath)
	}

	for k, v := range props {
		if err := target.SetProperty(k, v); err != nil {
			node.mu.Unlock()
			return err
		}
	}
	node.mu.Unlock()

	ct.EventBus.Emit(Event{
		Path:  listPath,
		Type:  EventUpdate,
		Entry: target,
	})

	return nil
}

// ListNodes returns all child node names at a given path.
func (ct *ConfigTree) ListNodes(path string) ([]string, error) {
	node, err := ct.Navigate(path)
	if err != nil {
		return nil, err
	}

	node.mu.RLock()
	defer node.mu.RUnlock()

	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	return names, nil
}
