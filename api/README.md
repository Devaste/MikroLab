# MikroLab WebSocket API Contract

This directory defines the **strict, versioned contract** for all WebSocket
messages exchanged between the frontend (React/TypeScript) and the backend
(Go) simulator.

## File Layout

```
api/
├── README.md                       This file
├── schemas/
│   ├── CommandRequest.json         Client → Server
│   ├── CommandResponse.json        Server → Client
│   └── EventMessage.json           Server → Client (async events)
```

## Schema as Single Source of Truth

The JSON Schema files in `api/schemas/` are the **single source of truth**
for the WebSocket message format. From these files:

- **Go structs** are generated with
  [go-jsonschema](https://github.com/atombender/go-jsonschema) via
  `//go:generate` directives in `internal/api/schema_gen.go`.
- **TypeScript interfaces** are generated with
  [json-schema-to-typescript](https://github.com/bcherny/json-schema-to-typescript)
  via `npm run generate:api` in the `frontend/` directory.
- **Runtime validation** on both sides uses the same schemas (via
  [jsonschema](https://github.com/santhosh-tekuri/jsonschema) in Go and
  [AJV](https://ajv.js.org/) in TypeScript).

## Versioning Policy

- The `version` field in every `CommandRequest` carries the API version
  string (e.g., `"1.0"`).
- Breaking changes (field removal, type change, required-field addition)
  **must** increment the **major** version number (e.g., `"1.0"` → `"2.0"`).
- Non-breaking additions (new optional fields, new event types) may
  increment the **minor** version number (e.g., `"1.0"` → `"1.1"`).
- The client sends its expected version on connect; if the server does not
  support it, the server responds with `{"error": "version_mismatch"}` and
  closes the connection.

## Regenerating Code

### Backend (Go)

```bash
go generate ./internal/api/...
```

Requires `go-jsonschema` – installed automatically via `go.mod`.

### Frontend (TypeScript)

```bash
cd frontend
npm run generate:api
```

Requires `json-schema-to-typescript` – listed in `frontend/package.json`.

## Message Lifecycle

1. **Connection handshake:** Client sends `{"version": "1.0"}` as the first
   message. Server validates the version.
2. **Command request:** Client sends a validated `CommandRequest` JSON
   object.
3. **Command response:** Server processes the command and replies with a
   `CommandResponse` whose `id` matches the request.
4. **Async events:** The server may push `EventMessage` objects at any time
   (e.g., log output, packet captures).
5. **Disconnect:** Either side may close the WebSocket at any time.