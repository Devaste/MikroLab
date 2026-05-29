// Package ping implements the /ping command for MikroLab.
// It simulates ICMP echo requests using route lookup and ARP resolution.
package ping

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Devaste/MikroLab/internal/core"
	"github.com/Devaste/MikroLab/internal/modules/ip/route"
)

// ---------------------------------------------------------------------------
// Dependency interfaces
// ---------------------------------------------------------------------------

// RouteLookuper provides route lookup for a destination IP.
type RouteLookuper interface {
	Lookup(dstIP string) *route.LookupResult
}

// ARPResolver provides MAC address resolution for an IP on a given interface.
type ARPResolver interface {
	Resolve(ip string, ifaceName string) (string, bool)
}

// ---------------------------------------------------------------------------
// PingCommand — implements core.Command
// ---------------------------------------------------------------------------

// PingCommand simulates ICMP echo requests using the routing table and ARP table.
// It does not store any persistent state.
type PingCommand struct {
	mu       sync.RWMutex
	path     string
	title    string
	routeMod RouteLookuper
	arpMod   ARPResolver
}

// New creates a new PingCommand.
func New(path string, title string, routeMod RouteLookuper, arpMod ARPResolver) (*PingCommand, error) {
	if path == "" {
		return nil, fmt.Errorf("ping: path is required")
	}
	if routeMod == nil {
		return nil, fmt.Errorf("ping: route module is required")
	}
	return &PingCommand{
		path:     path,
		title:    title,
		routeMod: routeMod,
		arpMod:   arpMod,
	}, nil
}

// ---------------------------------------------------------------------------
// core.Node interface
// ---------------------------------------------------------------------------

func (c *PingCommand) Path() string        { return c.path }
func (c *PingCommand) Type() core.NodeType { return core.NodeTypeSingleton }
func (c *PingCommand) Title() string       { return c.title }

// ---------------------------------------------------------------------------
// core.Directory interface (leaf node)
// ---------------------------------------------------------------------------

func (c *PingCommand) Children() map[string]core.Node { return nil }
func (c *PingCommand) AddChild(name string, child core.Node) error {
	return fmt.Errorf("ping: command node %q cannot accept child %q", c.path, name)
}
func (c *PingCommand) RemoveChild(name string) error {
	return fmt.Errorf("ping: command node %q has no child %q", c.path, name)
}
func (c *PingCommand) Child(name string) (core.Node, bool) { return nil, false }

// ---------------------------------------------------------------------------
// core.Command interface
// ---------------------------------------------------------------------------

// pingResult stores the result of a single ping probe.
type pingResult struct {
	seq     int
	rtt     time.Duration
	success bool
}

// Execute runs the ping command with the given arguments.
//
// Supported arguments:
//   - address (string, required) – destination IP address
//   - count (string, default "1") – number of ping packets (1-10)
//   - size (string, default "56") – packet size in bytes (ignored for simulation)
//   - interval (string, default "1s") – interval between pings (ignored for MVP)
//
// Returns a formatted string matching RouterOS ping output.
func (c *PingCommand) Execute(args map[string]interface{}) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Parse address (required)
	addressRaw, ok := args["address"]
	if !ok {
		return nil, fmt.Errorf("missing address")
	}
	address, ok := addressRaw.(string)
	if !ok || strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("missing address")
	}
	address = strings.TrimSpace(address)

	// 2. Parse count (default 1, cap at 10)
	count := 1
	if countRaw, ok := args["count"]; ok {
		if countStr, ok := countRaw.(string); ok && countStr != "" {
			if v, err := strconv.Atoi(countStr); err == nil && v >= 1 && v <= 10 {
				count = v
			}
		}
	}

	// 3. Parse size (default 56, for display only)
	size := 56
	if sizeRaw, ok := args["size"]; ok {
		if sizeStr, ok := sizeRaw.(string); ok && sizeStr != "" {
			if v, err := strconv.Atoi(sizeStr); err == nil && v > 0 {
				size = v
			}
		}
	}

	// 4. Perform route lookup
	routeResult := c.routeMod.Lookup(address)

	// 5. Run the pings
	results := make([]pingResult, count)
	sent := count
	received := 0

	for seq := 0; seq < count; seq++ {
		success := false

		if routeResult != nil {
			// Determine which MAC to resolve
			resolveIP := ""
			outIface := routeResult.OutInterface

			if routeResult.Gateway != "" && !isInterfaceName(routeResult.Gateway) {
				// Gateway is an IP (via router) — resolve gateway's MAC
				resolveIP = routeResult.Gateway
			} else {
				// Directly connected — resolve destination IP's MAC
				resolveIP = address
				if routeResult.OutInterface != "" {
					outIface = routeResult.OutInterface
				}
			}

			// Try to resolve MAC
			if c.arpMod != nil && resolveIP != "" {
				_, found := c.arpMod.Resolve(resolveIP, outIface)
				if found {
					success = true
				}
			} else {
				// No ARP module — assume reachable
				success = true
			}
		}

		// Simulate RTT: base = 1ms + (distance * 0.5ms) + random jitter (0-2ms)
		rtt := time.Duration(1) * time.Millisecond
		if routeResult != nil {
			rtt += time.Duration(int64(float64(routeResult.Distance) * 0.5 * float64(time.Millisecond)))
		}
		rtt += time.Duration(rand.Intn(3)) * time.Millisecond

		results[seq] = pingResult{
			seq:     seq,
			rtt:     rtt,
			success: success,
		}
		if success {
			received++
		}
	}

	// 6. Build output
	return formatPingOutput(address, size, results, sent, received), nil
}

// isInterfaceName checks if a string looks like an interface name (not an IP).
func isInterfaceName(s string) bool {
	return !strings.Contains(s, ".") && !strings.Contains(s, ":")
}

// ---------------------------------------------------------------------------
// Output formatting
// ---------------------------------------------------------------------------

// formatPingOutput produces RouterOS-style ping output.
func formatPingOutput(host string, size int, results []pingResult, sent, received int) string {
	var b strings.Builder

	// Header
	b.WriteString("SEQ HOST SIZE TTL TIME STATUS\n")

	// Each probe
	for _, r := range results {
		if r.success {
			rttStr := formatRTT(r.rtt)
			b.WriteString(fmt.Sprintf("%d %s %d 64 %s\n", r.seq, host, size, rttStr))
		} else {
			b.WriteString(fmt.Sprintf("%d %s %d -- -- timeout\n", r.seq, host, size))
		}
	}

	// Summary
	loss := 0
	if sent > 0 {
		loss = int(math.Round(float64(sent-received) / float64(sent) * 100))
	}

	// Calculate min/avg/max
	var minRTT, maxRTT, totalRTT time.Duration
	first := true
	for _, r := range results {
		if r.success {
			if first {
				minRTT = r.rtt
				maxRTT = r.rtt
				first = false
			}
			if r.rtt < minRTT {
				minRTT = r.rtt
			}
			if r.rtt > maxRTT {
				maxRTT = r.rtt
			}
			totalRTT += r.rtt
		}
	}

	if received > 0 {
		avgRTT := totalRTT / time.Duration(received)
		b.WriteString(fmt.Sprintf("sent=%d received=%d packet-loss=%d%% min-rtt=%s avg-rtt=%s max-rtt=%s\n",
			sent, received, loss, formatRTT(minRTT), formatRTT(avgRTT), formatRTT(maxRTT)))
	} else {
		b.WriteString(fmt.Sprintf("sent=%d received=%d packet-loss=%d%%\n", sent, received, loss))
	}

	return b.String()
}

// formatRTT formats a duration as a human-readable RTT string (e.g., "2ms", "1.5ms").
func formatRTT(d time.Duration) string {
	ms := float64(d) / float64(time.Millisecond)
	if ms == float64(int64(ms)) {
		return fmt.Sprintf("%dms", int(ms))
	}
	return fmt.Sprintf("%.1fms", ms)
}

// Ensure PingCommand implements core.Command
var _ core.Command = (*PingCommand)(nil)
