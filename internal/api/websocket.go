package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/Devaste/MikroLab/internal/topology"
	"github.com/gorilla/websocket"
)

// Supported API versions.
var supportedVersions = map[string]bool{
	"1.0": true,
}

// Supported version string for error messages.
const versionString = "1.0"

// WebSocket message types for wire format.
type (
	// rawRequest is the minimal parse of an incoming message to determine type.
	rawRequest struct {
		Version *string         `json:"version,omitempty"`
		ID      *int            `json:"id,omitempty"`
		Command *string         `json:"command,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	// rawResponse is the outgoing message format.
	rawResponse struct {
		ID     int              `json:"id"`
		Result *json.RawMessage `json:"result,omitempty"`
		Error  *string          `json:"error,omitempty"`
	}

	// handshakeMessage is the first message a client sends.
	handshakeMessage struct {
		Version string `json:"version"`
	}
)

// Server represents a WebSocket server for the MikroLab API.
type Server struct {
	addr      string
	upgrader  websocket.Upgrader
	compiler  *Compiler
	topology  *topology.Topology
	clients   map[*client]bool
	mu        sync.RWMutex
	nextID    atomic.Int64
	broadcast chan []byte
	done      chan struct{}
}

// client represents a single WebSocket connection.
type client struct {
	conn   *websocket.Conn
	server *Server
	send   chan []byte
}

// NewServer creates a new WebSocket API server.
func NewServer(addr string) (*Server, error) {
	compiler, err := NewCompiler()
	if err != nil {
		return nil, fmt.Errorf("failed to create schema compiler: %w", err)
	}

	return &Server{
		addr: addr,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins during development
			},
		},
		compiler:  compiler,
		topology:  topology.NewTopology(),
		clients:   make(map[*client]bool),
		broadcast: make(chan []byte, 256),
		done:      make(chan struct{}),
	}, nil
}

// SetTopology sets the topology manager for this server.
// Used to inject the topology from main.go after creating the default device.
func (s *Server) SetTopology(topo *topology.Topology) {
	s.topology = topo
}

// Topology returns the server's topology manager.
func (s *Server) Topology() *topology.Topology {
	return s.topology
}

// Start begins listening for WebSocket connections on the configured address.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","version":"`+versionString+`"}`)
	})

	server := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	log.Printf("WebSocket API server starting on %s", s.addr)

	// Start broadcast handler
	go s.broadcastHandler()

	return server.ListenAndServe()
}

// broadcastHandler sends messages to all connected clients.
func (s *Server) broadcastHandler() {
	for {
		select {
		case msg := <-s.broadcast:
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.send <- msg:
				default:
					// Client send buffer full, skip
				}
			}
			s.mu.RUnlock()
		case <-s.done:
			return
		}
	}
}

// Broadcast sends a message to all connected clients.
func (s *Server) Broadcast(msg []byte) {
	s.broadcast <- msg
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	close(s.done)
}

// handleWebSocket is the HTTP handler for WebSocket upgrades.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	c := &client{
		conn:   conn,
		server: s,
		send:   make(chan []byte, 256),
	}

	// Register client
	s.mu.Lock()
	s.clients[c] = true
	s.mu.Unlock()

	// Start read/write goroutines
	go c.writePump()
	go c.readPump()
}

// readPump reads messages from the WebSocket connection.
func (c *client) readPump() {
	defer func() {
		c.server.mu.Lock()
		delete(c.server.clients, c)
		c.server.mu.Unlock()
		c.conn.Close()
	}()

	// Helper to send an error directly (bypassing the send channel) and close.
	sendErrorAndClose := func(id int, errMsg string) {
		resp := rawResponse{
			ID:    id,
			Error: &errMsg,
		}
		data, _ := json.Marshal(resp)
		_ = c.conn.WriteMessage(websocket.TextMessage, data)
	}

	// Step 1: Read the version handshake
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		log.Printf("WebSocket read error during handshake: %v", err)
		return
	}

	// Parse the handshake
	var hs rawRequest
	if err := json.Unmarshal(msg, &hs); err != nil {
		sendErrorAndClose(-1, "invalid_json")
		return
	}

	// Validate the handshake has a version
	if hs.Version == nil {
		sendErrorAndClose(-1, "missing_version")
		return
	}

	// Check if version is supported
	if !supportedVersions[*hs.Version] {
		errMsg := fmt.Sprintf("version_mismatch: server supports %s", versionString)
		sendErrorAndClose(-1, errMsg)
		return
	}

	log.Printf("Client connected with API version %s", *hs.Version)

	// Step 2: Process incoming commands
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes a single incoming message.
func (c *client) handleMessage(msg []byte) {
	// Validate against CommandRequest schema
	schema := c.server.compiler.Get("CommandRequest.json")
	if schema == nil {
		c.sendError(-1, "internal_error: schema not loaded")
		return
	}

	// Parse as generic JSON for validation
	var raw interface{}
	if err := json.Unmarshal(msg, &raw); err != nil {
		c.sendError(-1, "invalid_json")
		return
	}

	// Validate against schema
	if err := schema.Validate(raw); err != nil {
		c.sendError(-1, fmt.Sprintf("validation_error: %v", err))
		return
	}

	// Parse into structured request
	var req struct {
		Version  string                 `json:"version"`
		ID       int                    `json:"id"`
		Command  string                 `json:"command"`
		Params   map[string]interface{} `json:"params,omitempty"`
		DeviceID string                 `json:"deviceId,omitempty"`
	}

	if err := json.Unmarshal(msg, &req); err != nil {
		c.sendError(-1, "invalid_json")
		return
	}

	// Handle topology commands separately
	if isTopologyCommand(req.Command) {
		result, err := c.executeTopologyCommand(req.Command, req.Params)
		if err != nil {
			c.sendError(req.ID, err.Error())
			return
		}
		c.sendResultJSON(req.ID, result)
		return
	}

	// Route regular RouterOS commands to the target device
	result, err := c.executeDeviceCommand(req.Command, req.Params, req.DeviceID)
	if err != nil {
		c.sendError(req.ID, err.Error())
		return
	}

	// Send the response
	resultStr, ok := result.(string)
	if !ok {
		resultStr = fmt.Sprintf("%v", result)
	}

	c.sendResult(req.ID, resultStr)
}

// isTopologyCommand checks if the command is a topology management command.
func isTopologyCommand(cmd string) bool {
	topoCommands := map[string]bool{
		"topology/create-device": true,
		"topology/delete-device": true,
		"topology/list-devices":  true,
		"topology/connect":       true,
		"topology/disconnect":    true,
		"topology/list-links":    true,
	}
	return topoCommands[cmd]
}

// executeTopologyCommand handles topology management commands.
func (c *client) executeTopologyCommand(cmd string, params map[string]interface{}) (interface{}, error) {
	topo := c.server.topology

	switch cmd {
	case "topology/create-device":
		name := "Router"
		if params != nil {
			if n, ok := params["name"]; ok {
				if nameStr, ok := n.(string); ok && nameStr != "" {
					name = nameStr
				}
			}
		}
		dev, err := topo.CreateDevice(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create device: %w", err)
		}
		return map[string]interface{}{
			"id":   dev.ID,
			"name": dev.Name,
		}, nil

	case "topology/delete-device":
		if params == nil {
			return nil, fmt.Errorf("params required with deviceId")
		}
		deviceID, ok := params["deviceId"].(string)
		if !ok || deviceID == "" {
			return nil, fmt.Errorf("deviceId parameter required")
		}
		if err := topo.DeleteDevice(deviceID); err != nil {
			return nil, err
		}
		return map[string]string{"status": "deleted", "deviceId": deviceID}, nil

	case "topology/list-devices":
		devices := topo.Devices()
		result := make([]map[string]string, 0, len(devices))
		for _, dev := range devices {
			result = append(result, map[string]string{
				"id":   dev.ID,
				"name": dev.Name,
			})
		}
		return result, nil

	case "topology/connect":
		if params == nil {
			return nil, fmt.Errorf("params required with deviceA, interfaceA, deviceB, interfaceB")
		}
		deviceA, _ := params["deviceA"].(string)
		ifaceA, _ := params["interfaceA"].(string)
		deviceB, _ := params["deviceB"].(string)
		ifaceB, _ := params["interfaceB"].(string)
		if deviceA == "" || ifaceA == "" || deviceB == "" || ifaceB == "" {
			return nil, fmt.Errorf("deviceA, interfaceA, deviceB, interfaceB are required")
		}
		if err := topo.Connect(deviceA, ifaceA, deviceB, ifaceB); err != nil {
			return nil, err
		}
		return map[string]string{"status": "connected"}, nil

	case "topology/disconnect":
		if params == nil {
			return nil, fmt.Errorf("params required with linkId")
		}
		linkID, ok := params["linkId"].(string)
		if !ok || linkID == "" {
			return nil, fmt.Errorf("linkId parameter required")
		}
		if err := topo.Disconnect(linkID); err != nil {
			return nil, err
		}
		return map[string]string{"status": "disconnected"}, nil

	case "topology/list-links":
		links := topo.Links()
		result := make([]map[string]string, 0, len(links))
		for _, link := range links {
			result = append(result, map[string]string{
				"id":         link.ID,
				"deviceA":    link.DeviceA,
				"interfaceA": link.InterfaceA,
				"deviceB":    link.DeviceB,
				"interfaceB": link.InterfaceB,
			})
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown topology command: %s", cmd)
	}
}

// executeDeviceCommand routes a RouterOS command to the appropriate device.
func (c *client) executeDeviceCommand(cmd string, params map[string]interface{}, deviceID string) (interface{}, error) {
	// Default to the first device if no deviceId specified
	if deviceID == "" {
		devices := c.server.topology.Devices()
		// Find the first device
		for _, dev := range devices {
			return dev.ExecuteAPI(cmd, params)
		}
		return nil, fmt.Errorf("no devices available")
	}

	dev := c.server.topology.Device(deviceID)
	if dev == nil {
		return nil, fmt.Errorf("device %q not found", deviceID)
	}

	return dev.ExecuteAPI(cmd, params)
}

// sendError sends an error response to the client.
func (c *client) sendError(id int, errMsg string) {
	resp := rawResponse{
		ID:    id,
		Error: &errMsg,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal error response: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("Client send buffer full, dropping error message")
	}
}

// sendResult sends a successful response to the client.
func (c *client) sendResult(id int, result string) {
	rawResult := json.RawMessage(`"` + result + `"`)
	resp := rawResponse{
		ID:     id,
		Result: &rawResult,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("Client send buffer full, dropping response")
	}
}

// sendResultJSON sends a JSON result response to the client.
func (c *client) sendResultJSON(id int, result interface{}) {
	resultBytes, err := json.Marshal(result)
	if err != nil {
		c.sendError(id, fmt.Sprintf("failed to marshal result: %v", err))
		return
	}

	rawResult := json.RawMessage(resultBytes)
	resp := rawResponse{
		ID:     id,
		Result: &rawResult,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal JSON response: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("Client send buffer full, dropping JSON response")
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
func (c *client) writePump() {
	defer c.conn.Close()

	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("WebSocket write error: %v", err)
			return
		}
	}
}
