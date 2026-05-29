# RouterOS Simulator – Blueprint v0.1

## Goal
Create a spiritual successor to Cisco Packet Tracer for MikroTik devices — a simulator that fully reproduces the behavior of RouterOS v7 (LTS) without using virtualization.

## Core Principles
- **Simulation, not emulation:** a packet engine mirrors the traffic path inside a single device and between devices.
- **Reflective Data Model:** CLI, GUI and WebFig are merely views of a single configuration tree.
- **Full CLI compatibility:** scripts written for real hardware must run unchanged.
- **Local server + web UI:** user launches a single binary, the UI is accessed via browser.
- **Open Source (AGPL v3).**

## Architecture

### Backend (Go)
- **Dependency management:** Go Modules (`go.mod` / `go.sum`) — standard `go get` / `go mod tidy` workflow.
- **Configuration core:** in-memory tree reflecting RouterOS structure (directories, lists, properties).
- **CLI parser and interpreter:** full support of RouterOS syntax (including nested commands, variables, `where` clauses).
- **Packet engine:** simulation of packet processing (ARP, IP, firewall, NAT, connection tracking, routing).
- **Topology:** graph of devices and links, delivering packets between ports.
- **API (WebSocket + versioning):** bidirectional communication with the UI – commands, tree updates, debug streams. All WebSocket messages (requests and responses) include a top-level `"version": "1.0"` field. The version is incremented on any breaking change to the message schema. The client sends its expected version on connect; if the server does not support it, it sends a `version_mismatch` error and closes the connection.

### Frontend (React + TypeScript)
- **Dependency management:** npm (`frontend/package.json` / `frontend/package-lock.json`) — standard `npm install` / `npm update` workflow.
- Separate project in the `frontend/` directory, Vite-powered build with TypeScript compilation.
- **Stack:** React 18 + TypeScript 5 + Vite 6 + React Flow + xterm.js
- **Canvas (React Flow):** visual network design.
- **Device popup windows:** CLI (xterm.js), GUI configurator (Webfig-like), possibly Winbox-like.
- **Debug tools:** sniffer, torch, ping (real-time via event stream).

### Modular System
- Module schemas described in JSON: path, node type, fields, validators, actions (add, set, remove, print, …).
- Modules loaded as plugins, enabling easy extension.
- Device profiles (JSON) define available ports, hardware capabilities, default behavior.

## Alpha Version (MVP)
- **Devices:** at least one profile (e.g., hAP ax³).
- **Configuration capabilities:**
  - `/ip address`
  - `/ip route` (static + connected)
  - `/ip firewall filter` (including default rule set)
  - `/ip firewall nat` (masquerade, src-nat, dst-nat)
  - `/interface bridge`, `/interface bridge port`
  - `/ip dhcp-server` (leases, make-static, options)
- **CLI:** full support for listed commands, tab completion, syntax highlighting, script execution.
- **GUI:** tabular display and editing of the same entities.
- **Topology:** create multiple devices, connect ports with cables.
- **Packet simulation:** ping between devices, ARP, traversal through firewall and NAT.
- **Project save/load** (YAML).

## Roadmap
- **v0.2 – Advanced features:** OSPF, VLAN, bonding, MPLS, VPLS, BGP.
- **v0.3 – Real device profiles** with hardware constraints.
- **v1.0 – Full RouterOS 7 LTS compatibility**, including wireless interfaces.

## Tech Stack
- **Core language:** Go 1.22+
- **Frontend Stack:** React 18 + TypeScript 5 (via `frontend/package.json` — Vite-powered)
  - React Flow 11 — network canvas
  - xterm.js 5 — terminal emulator
- **Communication:** WebSocket (JSON-RPC with API versioning), HTTP REST for export/import
  - **API Versioning:** Every WebSocket envelope includes a `version` field (e.g., `"1.0"`). Handshake on connect ensures client/server compatibility before any other message is exchanged. Bumped on breaking schema changes.
- **Testing:** Go testing + testify, scenario tests based on .rsc scripts
- **License:** AGPL v3

## License Compatibility

All third-party dependencies are permissively licensed and fully compatible with AGPLv3:

| Dependency | License | Compatible |
|---|---|---|
| **Backend (Go)** | | |
| `google/uuid` | BSD-3-Clause | ✅ |
| **Frontend runtime** | | |
| `react`, `react-dom` | MIT | ✅ |
| `reactflow` (React Flow 11) | MIT | ✅ |
| `xterm`, `xterm-addon-fit`, `xterm-addon-web-links` | MIT | ✅ |
| **Frontend dev** | | |
| `typescript` | Apache-2.0 | ✅ |
| `vite` | MIT | ✅ |
| `@vitejs/plugin-react` | MIT | ✅ |
| `eslint` + plugins | MIT | ✅ |
| `@typescript-eslint/*` | MIT / BSD-2-Clause | ✅ |
| `@types/react`, `@types/react-dom` | MIT | ✅ |

No GPL-family or copyleft libraries are used. AGPLv3's network-use clause (Section 13) requires that users interacting over a network can receive the source — this is satisfied by the open-source repository and will be documented in the README.md. Any future dependency additions must be audited against this table before inclusion.
