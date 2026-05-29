# Configuration Data Model

## Core Concepts

The entire configuration of a MikroTik device is represented as an in-memory tree. Every RouterOS path (e.g., `/ip address`, `/interface bridge vlan`) corresponds to a node in this tree. The tree is the **single source of truth** – all CLI commands, GUI forms, and WebFig actions ultimately read or modify this tree.

## Node Types

### Directory
A node that contains other nodes (sub‑directories or lists).  
Example: `/ip` is a directory containing `/ip address`, `/ip firewall`, etc.

### List
A node that holds an ordered collection of **entries**. Each entry has the same set of properties.  
Example: `/ip address` is a list of IP address entries.

### Entry (a row in a list)
An entry is a set of **property values**, plus a set of **flags**. Flags are boolean states like `disabled`, `dynamic`, `invalid`, `slave`.  
Entries are identified by their position (index) or by a unique `.id` (the router’s internal identifier). We emulate `.id` as a UUID string.

### Property
Each property has:
- **Name**
- **Type** (string, integer, boolean, IP address, IP prefix, enum, composite, etc.)
- **Required** flag
- **Default value** (if not provided)
- **Read‑only** flag (cannot be changed directly by user)
- **Validation rules** (regex, range, dependency on other fields)

### Computed Property
Some fields are not stored directly but are calculated from other properties.  
Example: `network` and `broadcast` in `/ip address` are derived from the `address` prefix. These are cached internally and re‑computed when the source property changes.

## Flag System

Flags are boolean markers attached to each entry. They are not part of the entry’s properties but are always visible in CLI output (the `X`, `I`, `D`, `S` columns).  
- **disabled** – the entry is inactive (can be toggled by `disable`/`enable` commands).  
- **invalid** – the entry has invalid configuration (e.g., missing interface). Cannot be cleared manually.  
- **dynamic** – the entry was created by a DHCP client or other service. Cannot be deleted by the user directly (needs to be released by the service).  
- **slave** – indicates that the entry’s interface is a slave port to a master interface. Set automatically.

## Reflective Model

Every interaction (CLI, GUI, script) generates an **abstract operation** that is applied to the tree. After a successful apply, the tree notifies all connected clients (frontend, CLI sessions) about the delta.

Operations:
- `add` – insert a new entry with given properties, apply defaults for missing ones, run validators.
- `set` – modify one or more properties of existing entries (by number, `.id`, or `where` filter).
- `remove` – delete entries.
- `move` – reorder entries in a list.
- `print` – query entries, output formatted.
- `export` – generate CLI commands that reproduce the current configuration.

## Two‑Phase Apply

1. **Command parsing** – the CLI parser translates the input string into an operation object.
2. **Validation** – the operation is checked against the module’s schema, cross‑field constraints, and existing data (e.g., duplicate IP on same interface → error).
3. **Apply** – the tree is modified, computed properties are updated, and events are emitted.
4. **Error** – if validation fails, an error message is returned, and nothing is changed (no partial apply).

## Cross‑Module References

Properties may reference entities in other modules (e.g., `interface` in `/ip address` refers to an interface name in `/interface`). The data model maintains referential integrity:
- If an interface is renamed, all referencing addresses automatically update the interface name.
- If an interface is deleted, the address becomes `invalid` (flag set, read‑only property cleared).

## Example: `/ip address` Entry

Flags: `X` – disabled, `I` – invalid, `D` – dynamic, `S` – slave

```
 0 address=192.168.1.1/24 network=192.168.1.0 broadcast=192.168.1.255 interface=ether1
```

This entry is stored internally as:
- `.id` = `"uuid-1"`
- `address` = `"192.168.1.1/24"`
- `interface` = `"ether1"`
- `network` = `"192.168.1.0"` (computed)
- `broadcast` = `"192.168.1.255"` (computed)
- Flags: none (all off)

## Default Values

Each module defines default property values. When an entry is added without specifying a property, the default is used.  
For `/ip address`, the only mandatory fields are `address` and `interface`; others are either computed or have a default (e.g., `comment=""`).

## Event System

Every tree modification emits an event with:
- Path of the changed list/directory
- Type of change (add/remove/update/move)
- Full entry state after change

WebSocket clients receive these events and update their UI accordingly.