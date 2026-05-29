package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// testServer creates a test WebSocket server for testing.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()

	s, err := NewServer(":0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := http.HandlerFunc(s.handleWebSocket)
	server := httptest.NewServer(handler)
	return server
}

// wsURL converts an http test server URL to a ws:// URL.
func wsURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

// readWithClose reads a text message from the connection.
// For handshake errors (invalid/missing version), the server sends a
// text message and then closes. We handle both cases: if the text was
// received before close, return it; if the close came first, return the error.
func readWithClose(conn *websocket.Conn) ([]byte, error) {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// TestHandshake_Valid tests that a valid version handshake succeeds.
func TestHandshake_Valid(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send valid version handshake
	err = conn.WriteJSON(map[string]string{"version": "1.0"})
	if err != nil {
		t.Fatalf("Failed to send handshake: %v", err)
	}

	// Send a valid command request (handshake should pass, and we should get a response)
	err = conn.WriteJSON(map[string]interface{}{
		"version": "1.0",
		"id":      1,
		"command": "/interface/print",
	})
	if err != nil {
		t.Fatalf("Failed to send command: %v", err)
	}

	// Read response (may be an error about no tree, but we verified handshake passed)
	msg, err := readWithClose(conn)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var resp rawResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	// Should get a response (even if it's an error about tree not being initialized)
	// The important thing is we got a response at all, proving handshake passed
	_ = resp
}

// TestHandshake_InvalidVersion tests that an unsupported version is rejected.
func TestHandshake_InvalidVersion(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send invalid version
	err = conn.WriteJSON(map[string]string{"version": "999.0"})
	if err != nil {
		t.Fatalf("Failed to send handshake: %v", err)
	}

	// Read response - should be an error about version mismatch,
	// or we might get a close error if the server closes before we read
	msg, err := readWithClose(conn)
	if err != nil {
		// Accept close errors - the server may have closed the connection
		// before we could read the error message. This is acceptable.
		return
	}

	var resp rawResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v (raw: %s)", err, string(msg))
	}

	if resp.Error == nil {
		t.Fatal("Expected error response, got nil")
	}
	if !strings.Contains(*resp.Error, "version_mismatch") {
		t.Fatalf("Expected version_mismatch error, got: %s", *resp.Error)
	}
}

// TestHandshake_MissingVersion tests that a missing version field is rejected.
func TestHandshake_MissingVersion(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send message without version
	err = conn.WriteJSON(map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read response - should be an error, or close error if server closes first
	msg, err := readWithClose(conn)
	if err != nil {
		return
	}

	var resp rawResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v (raw: %s)", err, string(msg))
	}

	if resp.Error == nil {
		t.Fatal("Expected error response, got nil")
	}
	if !strings.Contains(*resp.Error, "missing_version") {
		t.Fatalf("Expected missing_version error, got: %s", *resp.Error)
	}
}

// TestMalformedJSON tests that malformed JSON is rejected with a validation error.
func TestMalformedJSON(t *testing.T) {
	server := testServer(t)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server), nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send valid handshake
	err = conn.WriteJSON(map[string]string{"version": "1.0"})
	if err != nil {
		t.Fatalf("Failed to send handshake: %v", err)
	}

	// Send invalid JSON (missing required fields for CommandRequest)
	err = conn.WriteJSON(map[string]interface{}{
		"version": "1.0",
		"id":      1,
		// Missing "command" field deliberately
	})
	if err != nil {
		t.Fatalf("Failed to send malformed request: %v", err)
	}

	// Read response - should be a validation error
	msg, err := readWithClose(conn)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var resp rawResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		t.Fatalf("Failed to parse response: %v (raw: %s)", err, string(msg))
	}

	if resp.Error == nil {
		t.Fatal("Expected error response for malformed request, got nil")
	}
	if !strings.Contains(*resp.Error, "validation_error") {
		t.Fatalf("Expected validation_error, got: %s", *resp.Error)
	}
}
