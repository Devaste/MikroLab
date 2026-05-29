# RouterOS Simulator (ROS Lab)

**A spiritual successor to Cisco Packet Tracer for MikroTik.**

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

> **Project status:** Pre-Alpha (active architecture design)

## About
The goal is to create a network simulator for MikroTik devices that fully reproduces RouterOS v7 LTS behavior. You will be able to:
- Design networks in a visual editor (like Packet Tracer).
- Configure virtual routers via the familiar CLI (identical to the real one), web interface, or GUI.
- Observe packet flow, use built-in sniffers and debugging tools.
- Write and test automation scripts without risking real hardware.

## Key Features
- **Reflective Data Model:** single configuration tree, CLI and GUI stay in sync.
- **Packet simulation:** packet processing inside a virtual device mirrors real RouterOS.
- **Modular architecture:** new protocols and features are added as plugins.
- **Local execution:** no external servers needed; runs on your machine, browser-based UI.
- **Real script compatibility:** import `.rsc` from production and verify behavior.

## Quick Start (available in the first alpha)
```bash
# Clone the repo
git clone https://github.com/your-org/routeros-sim.git
cd routeros-sim

# Run the server (port 8080 by default)
go run ./cmd/simulator

# Open your browser
http://localhost:8080
```

## Contributing
We welcome contributions! We especially need help with:
- Reverse engineering RouterOS behavior (testing CLI on real devices).
- Creating JSON schemas for new modules.
- Testing and writing scenario scripts.

See CONTRIBUTING.md (coming soon).

## License
Distributed under the GNU Affero General Public License v3. You may freely use, modify, and distribute the code, but must open source derivative works.