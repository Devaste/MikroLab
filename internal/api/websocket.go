package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/Devaste/MikroLab/internal/cli"
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
		clients:   make(map[*client]bool),
		broadcast: make(chan []byte, 256),
		done:      make(chan struct{}),
	}, nil
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
		Version string                 `json:"version"`
		ID      int                    `json:"id"`
		Command string                 `json:"command"`
		Params  map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.Unmarshal(msg, &req); err != nil {
		// This shouldn't happen after validation, but handle defensively
		c.sendError(-1, "invalid_json")
		return
	}

	// Execute the command
	result, err := cli.ExecuteAPI(req.Command, req.Params)
	if err != nil {
		errStr := err.Error()
		c.sendError(req.ID, errStr)
		return
	}

	// Send the response
	resultStr, ok := result.(string)
	if !ok {
		resultStr = fmt.Sprintf("%v", result)
	}

	c.sendResult(req.ID, resultStr)
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
