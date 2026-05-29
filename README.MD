<p align="center">
  <img src="docs/logo.png" alt="MikroLab logo" width="120"/>
</p>

<h1 align="center">MikroLab</h1>
<p align="center">
  <strong>A spiritual successor to Cisco Packet Tracer for MikroTik devices.</strong>
</p>

<p align="center">
  <a href="https://github.com/Devaste/MikroLab/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/License-AGPL_v3-blue.svg" alt="License: AGPL v3"/>
  </a>
  <img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8.svg?logo=go" alt="Go 1.22+"/>
  <img src="https://img.shields.io/badge/Frontend-React%2018%20%2B%20TS%205-61DAFB.svg?logo=react" alt="React 18 + TypeScript 5"/>
  <img src="https://img.shields.io/badge/Status-Pre--Alpha-yellow" alt="Pre-Alpha"/>
</p>

---

**Topics:** `network-simulator` `mikrotik` `routeros` `packet-tracer` `networking` `virtual-lab` `simulation`

---

## About

MikroLab is a full-featured network simulator for MikroTik devices that reproduces RouterOS v7 LTS behavior **without virtualization**. It uses a pure simulation engine written in Go, paired with a modern web frontend (React + TypeScript) for visual topology design and device management.

Think of it as **Cisco Packet Tracer, but for MikroTik** — design complex network topologies, configure virtual routers using the **real CLI**, watch packets traverse through firewall rules and NAT, and automate workflows with production `.rsc` scripts, all from a single binary on your machine.

### What you'll be able to do:

- **Design networks visually** — drag devices, draw cables, build topologies in a React Flow canvas.
- **Configure via real RouterOS CLI** — identical syntax, tab completion, script execution.
- **Use WebFig-like GUI** — tabular editors for all configuration entities.
- **Simulate packet flow** — ARP, IP routing, firewall, NAT, connection tracking.
- **Debug in real time** — built-in sniffer, torch, ping with live event streaming.
- **Automate** — write and test RouterOS scripts safely before deploying to production hardware.
- **Save & share** — export/import projects in YAML format.

## Project Status

> **Pre-Alpha** — Architecture design phase. See [`BLUEPRINT.MD`](BLUEPRINT.MD) for the full plan and current progress.

| Component | Status |
|---|---|
| Backend (Go) | Architecture defined, initial modules scaffolded |
| Frontend (React + TS) | Scaffolded (`frontend/package.json`), structure planned |
| API (WebSocket) | Versioning strategy documented |
| MVP features | Roadmap defined in BLUEPRINT.MD |

## Architecture Overview

### Module Directory Layout

The backend module hierarchy mirrors the RouterOS path structure. Each configuration module lives under `internal/modules/` and is organized by its RouterOS path:

```
internal/modules/
├── ip/
│   ├── address/            # implements /ip address
│   │   └── schema.json
│   ├── arp/                # implements /ip arp
│   │   └── schema.json
│   └── (future: cloud/, route/, firewall/, ...)
├── interfaces/             # implements /interface (top-level; will be moved under / later)
└── system/                 # implements /system (planned)
```

> **Note:** The filesystem path is purely for code organization — the module's `Path()` method returns the runtime RouterOS path (e.g., `/ip/address`), which is used by `GetNode()` for tree traversal and is **independent** of where the `.go` files are stored.

### System Architecture

```
┌─────────────────────────────────────────────┐
│              Browser (React UI)             │
│  ┌─────────┐ ┌──────────┐ ┌──────────────┐  │
│  │ Canvas  │ │ CLI      │ │ GUI / WebFig │  │
│  │ (React  │ │ (xterm)  │ │ (React Flow) │  │
│  │  Flow)  │ │          │ │              │  │
│  └────┬────┘ └────┬─────┘ └──────┬───────┘  │
│       └───────────┼──────────────┘          │
└───────────────────┼─────────────────────────┘
                    │ WebSocket (JSON-RPC + API versioning)
┌───────────────────┼──────────────────────────┐
│  ┌────────────────┴──────────────────────┐   │
│  │         Go Backend Server             │   │
│  │  ┌──────────┐  ┌──────────────────┐   │   │
│  │  │ CLI      │  │ Configuration    │   │   │
│  │  │ Parser   │  │ Tree (in-memory) │   │   │
│  │  └──────────┘  └──────────────────┘   │   │
│  │  ┌──────────┐  ┌──────────────────┐   │   │
│  │  │ Packet   │  │ Module System    │   │   │
│  │  │ Engine   │  │ (Plugins)        │   │   │
│  │  └──────────┘  └──────────────────┘   │   │
│  │  ┌──────────────────────────────┐     │   │
│  │  │  Topology Graph + Links      │     │   │
│  │  └──────────────────────────────┘     │   │
│  └───────────────────────────────────────┘   │
└──────────────────────────────────────────────┘
```

## Tech Stack

| Layer | Technology |
|---|---|
| **Backend** | Go 1.22+ |
| **Frontend** | React 18 + TypeScript 5 (Vite 6) |
| **UI Canvas** | React Flow 11 |
| **Terminal** | xterm.js 5 |
| **API** | WebSocket (JSON-RPC with versioning) + HTTP REST |
| **Dependency Management** | Go Modules (backend `go.mod`), npm (frontend `frontend/package.json`) |
| **Testing** | Go testing + testify + `.rsc` scenario tests |
| **License** | [AGPL v3](./LICENSE) |

See [`frontend/package.json`](frontend/package.json) for frontend dependencies (managed with `npm install` / `npm update`) and [`go.mod`](go.mod) for Go dependencies (managed with `go get` / `go mod tidy`). Full third-party license details in [`LICENSE-3RD-PARTY.MD`](LICENSE-3RD-PARTY.MD).

## Quick Start

> Available when the first alpha is released.

```bash
# Clone the repo
git clone https://github.com/Devaste/MikroLab.git
cd MikroLab

# Run the server (default port 8080)
go run ./cmd/simulator

# Open your browser
open http://localhost:8080
```

## Documentation

| Document | Description |
|---|---|
| [`BLUEPRINT.MD`](BLUEPRINT.MD) | Full architecture blueprint, MVP scope, and roadmap |
| [`docs/data-model.md`](docs/data-model.md) | Reflective data model design |
| [`LICENSE-3RD-PARTY.MD`](LICENSE-3RD-PARTY.MD) | Third-party dependency license audit |

## Contributing

We welcome contributions! Areas where help is most needed:

- **Reverse engineering** — test RouterOS behavior on real hardware and document edge cases.
- **Module schemas** — write JSON schemas for RouterOS modules (firewall, routing, DHCP, etc.).
- **Testing** — create `.rsc` scenario scripts and integration tests.
- **Frontend** — implement the React UI (canvas, terminal, config panels).

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for detailed contribution guidelines.

## Related Projects

- [EVE-NG](https://www.eve-ng.net/) — multi-vendor network emulator
- [GNS3](https://www.gns3.com/) — graphical network simulator
- [Cisco Packet Tracer](https://www.netacad.com/courses/packet-tracer) — inspiration for this project

## License

Distributed under the **GNU Affero General Public License v3**. See [`LICENSE`](LICENSE) for details.

You may freely use, modify, and distribute the code, but any modified version running as a network service must also make its source code available to users (AGPLv3 §13).
