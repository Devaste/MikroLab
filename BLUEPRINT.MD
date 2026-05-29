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
- **Configuration core:** in-memory tree reflecting RouterOS structure (directories, lists, properties).
- **CLI parser and interpreter:** full support of RouterOS syntax (including nested commands, variables, `where` clauses).
- **Packet engine:** simulation of packet processing (ARP, IP, firewall, NAT, connection tracking, routing).
- **Topology:** graph of devices and links, delivering packets between ports.
- **API (WebSocket):** bidirectional communication with the UI – commands, tree updates, debug streams.

### Frontend (React)
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
- **Frontend:** React 18+, TypeScript, React Flow, xterm.js
- **Communication:** WebSocket (JSON-RPC), HTTP REST for export/import
- **Testing:** Go testing + testify, scenario tests based on .rsc scripts
- **License:** AGPL v3