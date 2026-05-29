// Package log implements the /log command for MikroLab.
// It maintains a circular buffer of log entries accessible via /log print.
package log

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Devaste/MikroLab/internal/core"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// MaxLogEntries is the maximum number of log entries to keep in the buffer.
const MaxLogEntries = 1000

// ---------------------------------------------------------------------------
// LogModule — implements core.Command for /log
// ---------------------------------------------------------------------------

// LogModule maintains a circular buffer of log entries.
type LogModule struct {
	mu      sync.RWMutex
	path    string
	title   string
	buffer  []string
	maxSize int
	nextPos int // next write position in the circular buffer
	count   int // total entries written (for display)
}

// New creates a new LogModule.
func New(path, title string) (*LogModule, error) {
	if path == "" {
		return nil, fmt.Errorf("log: path is required")
	}
	return &LogModule{
		path:    path,
		title:   title,
		buffer:  make([]string, MaxLogEntries),
		maxSize: MaxLogEntries,
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

// Path returns the full absolute path "/log".
func (m *LogModule) Path() string { return m.path }

// Type returns core.NodeTypeSingleton.
func (m *LogModule) Type() core.NodeType { return core.NodeTypeSingleton }

// Title returns the human-readable display name.
func (m *LogModule) Title() string { return m.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

// Children returns nil — /log is a leaf node.
func (m *LogModule) Children() map[string]core.Node { return nil }

// AddChild returns an error — /log cannot contain children.
func (m *LogModule) AddChild(name string, child core.Node) error {
	return fmt.Errorf("log: command node %q cannot accept child %q", m.path, name)
}

// RemoveChild returns an error — /log has no children.
func (m *LogModule) RemoveChild(name string) error {
	return fmt.Errorf("log: command node %q has no child %q", m.path, name)
}

// Child returns (nil, false) — /log has no child nodes.
func (m *LogModule) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.Command interface
// ---------------------------------------------------------------------------

// Execute runs the /log command.
// Supported actions:
//   - "print" (or no args) — prints all stored log entries
func (m *LogModule) Execute(args map[string]interface{}) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.formatLogOutput(), nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Add adds a log entry to the circular buffer.
func (m *LogModule) Add(entry string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	timestamp := time.Now().Format("15:04:05")
	formatted := fmt.Sprintf("%s %s", timestamp, entry)

	m.buffer[m.nextPos] = formatted
	m.nextPos = (m.nextPos + 1) % m.maxSize
	m.count++
}

// GetEntries returns a copy of all log entries in chronological order.
func (m *LogModule) GetEntries() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.getEntries()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// getEntries returns log entries in chronological order.
// Caller must hold at least a read lock.
func (m *LogModule) getEntries() []string {
	total := m.count
	if total > m.maxSize {
		total = m.maxSize
	}

	entries := make([]string, total)

	if m.count <= m.maxSize {
		// Buffer hasn't wrapped yet — entries are at positions [0, count)
		for i := 0; i < total; i++ {
			if m.buffer[i] != "" {
				entries[i] = m.buffer[i]
			}
		}
	} else {
		// Buffer has wrapped — start at nextPos, then wrap around
		for i := 0; i < total; i++ {
			pos := (m.nextPos + i) % m.maxSize
			if m.buffer[pos] != "" {
				entries[i] = m.buffer[pos]
			}
		}
	}

	return entries
}

// formatLogOutput formats log entries for display.
func (m *LogModule) formatLogOutput() string {
	entries := m.getEntries()
	if len(entries) == 0 {
		return "No log entries.\n"
	}

	var b strings.Builder
	for _, entry := range entries {
		if entry != "" {
			b.WriteString(entry)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Ensure LogModule implements core.Command
var _ core.Command = (*LogModule)(nil)
