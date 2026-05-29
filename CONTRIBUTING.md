# Contributing to MikroLab

Thank you for your interest in contributing to MikroLab! This project aims to be a full RouterOS v7 LTS simulator, and we need help from the community — especially from MikroTik enthusiasts, network engineers, and Go/TypeScript developers.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How to Contribute](#how-to-contribute)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Contribution Areas](#contribution-areas)
  - [Reverse Engineering RouterOS](#reverse-engineering-routeros)
  - [Writing Module Schemas](#writing-module-schemas)
  - [Backend (Go)](#backend-go)
  - [Frontend (React + TypeScript)](#frontend-react--typescript)
  - [Testing](#testing)
  - [Documentation](#documentation)
- [Coding Standards](#coding-standards)
- [Pull Request Process](#pull-request-process)
- [License](#license)

---

## Code of Conduct

By participating in this project, you agree to abide by the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Be respectful, constructive, and inclusive.

---

## How to Contribute

1. **Check the roadmap** in [`BLUEPRINT.MD`](BLUEPRINT.MD) to see what's planned for the current milestone.
2. **Look for issues** tagged `help wanted` or `good first issue` in the GitHub issue tracker.
3. **Discuss major changes** by opening a discussion or issue before starting work to avoid duplication of effort.
4. **Fork the repository** and create a feature branch from `main`.
5. **Submit a pull request** once your work is ready for review.

---

## Getting Started

### Prerequisites

- **Go 1.22+** — [Install Go](https://go.dev/dl/)
- **Node.js 20+** — [Install Node.js](https://nodejs.org/) (for frontend development)
- **Git**

### Clone and Build

```bash
git clone https://github.com/Devaste/MikroLab.git
cd MikroLab

# Build the backend
go build ./cmd/simulator

# Install frontend dependencies (when working on the UI)
cd frontend
npm install
```

### Run Tests

```bash
# Backend tests
go test ./...

# Frontend lint
cd frontend && npm run lint
```

---

## Development Workflow

1. **Pick a task** — find an open issue or propose a new one.
2. **Create a branch** — `git checkout -b feat/my-feature` or `fix/my-bugfix`.
3. **Make changes** — follow the [coding standards](#coding-standards).
4. **Test** — ensure existing tests pass and add new ones for your changes.
5. **Commit** — use clear, descriptive commit messages (e.g., `feat: add /ip firewall filter module`).
6. **Push** — `git push origin your-branch`.
7. **Open a PR** — against the `main` branch with a description of your changes.

---

## Contribution Areas

### Reverse Engineering RouterOS

This is one of the most valuable ways to contribute. We need accurate behavioral data to make the simulator behave like real RouterOS.

**What to do:**

1. Set up a real MikroTik device (or CHR / x86 image) running RouterOS v7 LTS.
2. Execute commands and document the output, edge cases, and error messages.
3. Test CLI behavior for modules listed in the MVP (see [`BLUEPRINT.MD`](BLUEPRINT.MD)).
4. Save `.rsc` scripts that demonstrate specific behaviors.

**How to submit:**

- Create a `.rsc` script file in `internal/modules/` under the relevant module directory (e.g., `internal/modules/ip/firewall/filter/tests/`).
- Add a markdown document explaining the behavior, expected output, and any quirks observed.

**Example:**

```rsc
# /ip firewall filter test: verify default drop counter
/ip firewall filter print stats
```

Include the raw output from the real device alongside your analysis.

---

### Writing Module Schemas

Module schemas define how RouterOS entities map to the simulation's configuration tree.

**Schema location:** `internal/modules/<module-path>/schema.json`

**Schema structure:**

```json
{
  "path": "/ip firewall filter",
  "nodeType": "list",
  "fields": [
    {
      "name": "chain",
      "type": "string",
      "required": true,
      "values": ["input", "output", "forward"]
    },
    {
      "name": "src-address",
      "type": "ip_prefix",
      "required": false
    },
    {
      "name": "action",
      "type": "string",
      "values": ["accept", "drop", "reject", "jump"]
    }
  ],
  "validators": {
    "chain": "validateChain",
    "src-address": "validateIPPrefix"
  },
  "actions": ["add", "set", "remove", "print", "enable", "disable", "move"]
}
```

**Rules:**

- Types must match one of the supported primitives (`string`, `integer`, `ip`, `ip_prefix`, `mac`, `bool`, `enum`).
- Validators reference Go functions in the corresponding `validators.go` file.
- Include test schemas alongside real schemas for the testing framework.

---

### Backend (Go)

**Areas to work on:**

| Area | Where | Description |
|---|---|---|
| CLI Parser | `internal/config/` | Tokenizer, parser, interpreter for RouterOS syntax |
| Packet Engine | `internal/modules/packet/` | ARP, IP forwarding, firewall, NAT, connection tracking |
| Configuration Core | `internal/config/` | In-memory tree, property validation, change events |
| Module System | `internal/modules/` | Plugin loader, schema registry, action dispatcher |
| WebSocket API | `internal/api/` | JSON-RPC messages, session management, API versioning |
| Topology | `internal/topology/` | Device graph, link simulation, packet delivery |

**Guidelines:**

- Follow [Go's official style guide](https://go.dev/doc/effective_go) and use `gofmt` before committing.
- Use table-driven tests wherever possible.
- New modules should be self-contained: a schema, a Go implementation, and tests.

---

### Frontend (React + TypeScript)

**Areas to work on:**

| Area | Where | Description |
|---|---|---|
| Canvas | `frontend/src/canvas/` | React Flow topology editor (drag & drop devices, cables) |
| Terminal | `frontend/src/terminal/` | xterm.js wrapper with syntax highlighting |
| Config Panels | `frontend/src/panels/` | Tabular editors for configuration entities |
| WebSocket Client | `frontend/src/api/` | Connection management, message versioning, reconnection |
| Debug Tools | `frontend/src/debug/` | Sniffer, torch, ping visualizations |

**Guidelines:**

- Use **TypeScript** — no `any` unless absolutely necessary with a comment explaining why.
- Components should be functional with hooks (no class components).
- Styles via CSS modules or inline — no CSS-in-JS libraries unless discussed.
- Run `npm run lint` and `npm run build` before pushing.

**Project setup:**

```bash
cd frontend
npm install
npm run dev     # starts Vite dev server on localhost:5173
npm run build   # production build
```

---

### Testing

We use a layered testing strategy:

| Layer | Tool | What to Test |
|---|---|---|
| Unit | Go `testing` + `testify` | Individual functions, validators, parsers |
| Integration | Go `testing` + custom harness | End-to-end module behavior with `.rsc` scripts |
| Scenario | `.rsc` files + golden output files | Full command sequences matching real device output |
| Frontend | Vitest (planned) | React component rendering, state management |

**Writing scenario tests:**

1. Create a `.rsc` script in the module's `tests/` directory.
2. Create an expected output file with the same name but `.golden` extension.
3. The test harness runs the script through the simulator and compares output to the golden file.

Example:

```
internal/modules/ip/firewall/filter/tests/
├── basic-filter.rsc
├── basic-filter.golden
├── chain-jump.rsc
└── chain-jump.golden
```

Contributing `.rsc` + `.golden` pairs from real device testing is extremely valuable.

---

### Documentation

Good documentation is critical. Help us with:

- **Module docs** — what each module does, which commands it supports, known gaps vs real RouterOS.
- **Reverse engineering reports** — detailed comparisons between simulated and real behavior.
- **User guides** — how to set up common topologies, troubleshoot, etc.
- **Developer guides** — how to add a new module, extend the packet engine, etc.

Documentation goes in the `docs/` directory in Markdown format.

---

## Coding Standards

### Go

- Format with `gofmt` (enforced by CI).
- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments).
- Use `camelCase` for unexported, `PascalCase` for exported identifiers.
- Error handling: return errors, don't panic (except in tests for expected failures).
- Use `// PackageName` doc comments on packages.

### TypeScript / React

- Format with ESLint + Prettier (config in `frontend/`).
- Use TypeScript strict mode.
- Prefer `const` over `let` — no `var`.
- Name files in `kebab-case.tsx` for components, `camelCase.ts` for utilities.
- Export components as named exports, not defaults.

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add /ip firewall filter module
fix: correct ARP table timeout calculation
docs: add reverse engineering notes for /ip dhcp-server
test: add scenario tests for NAT masquerade
chore: update go dependencies
```

### Branch Naming

- `feat/<short-description>` — new features
- `fix/<short-description>` — bug fixes
- `docs/<short-description>` — documentation
- `test/<short-description>` — testing
- `chore/<short-description>` — maintenance

---

## Pull Request Process

1. **Ensure tests pass** — run `go test ./...` and `cd frontend && npm run lint && npm run build`.
2. **Update documentation** if your changes affect the API, module behavior, or setup process.
3. **Add or update tests** — aim for at least 80% coverage on new code.
4. **Link the issue** your PR addresses (e.g., "Closes #42").
5. **Describe your changes** — what was changed, why, and how to test it.
6. **Request review** from maintainers.
7. **Address feedback** — iterate until the PR is approved.

### PR Checklist

- [ ] Code compiles without errors
- [ ] Existing tests pass
- [ ] New tests cover my changes
- [ ] Documentation updated (if applicable)
- [ ] Commits follow Conventional Commits format
- [ ] Branch is up to date with `main`

---

## License

By contributing, you agree that your contributions will be licensed under the [AGPL v3](LICENSE) — the same license as the project.

---

*Thank you for helping make MikroLab a reality!*