// Package api provides the WebSocket server and generated JSON Schema bindings
// for the MikroLab API contract.
package api

//go:generate go run github.com/atombender/go-jsonschema -pkg api -o command_request_gen.go ../../api/schemas/CommandRequest.json
//go:generate go run github.com/atombender/go-jsonschema -pkg api -o command_response_gen.go ../../api/schemas/CommandResponse.json
//go:generate go run github.com/atombender/go-jsonschema -pkg api -o event_message_gen.go ../../api/schemas/EventMessage.json
