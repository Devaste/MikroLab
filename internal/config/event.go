package config

import (
	"sync"
)

// EventType represents the type of a tree modification event.
type EventType string

const (
	EventAdd    EventType = "add"
	EventRemove EventType = "remove"
	EventUpdate EventType = "update"
	EventMove   EventType = "move"
)

// Event represents a modification event in the config tree.
type Event struct {
	Path  string    `json:"path"`
	Type  EventType `json:"type"`
	Entry *Entry    `json:"entry,omitempty"`
}

// EventHandler is a callback for tree events.
type EventHandler func(event Event)

// EventBus manages event subscriptions and emissions.
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]EventHandler
	global    []EventHandler
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]EventHandler),
		global:    make([]EventHandler, 0),
	}
}

// Subscribe registers a handler for events on a specific path.
func (eb *EventBus) Subscribe(path string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if path == "" || path == "*" {
		eb.global = append(eb.global, handler)
		return
	}

	eb.listeners[path] = append(eb.listeners[path], handler)
}

// Unsubscribe removes all handlers for a path.
func (eb *EventBus) Unsubscribe(path string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.listeners, path)
}

// Emit sends an event to all matching listeners.
func (eb *EventBus) Emit(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Call path-specific listeners
	if handlers, ok := eb.listeners[event.Path]; ok {
		for _, h := range handlers {
			h(event)
		}
	}

	// Call global listeners
	for _, h := range eb.global {
		h(event)
	}
}
