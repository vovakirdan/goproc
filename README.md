# goproc

`goproc` is a small UNIX process watcher made of two pieces:

1. **Daemon** — a background service reachable over a per-user UNIX socket. It keeps a registry of processes, persists JSON snapshots, and periodically probes PIDs to mark them alive/dead.
2. **CLI** — a thin gRPC client that talks to the daemon and exposes user-friendly commands (`add`, `list`, `tag`, …).

The daemon keeps secondary indexes by ID, PID, **name**, tag, and group, so queries stay fast even as the registry grows. Each entry receives a monotonic `uint64` ID that survives restarts thanks to snapshots stored near the socket directory. Names are optional but must be unique—think of them as a human-friendly alias distinct from tags.

---

## Build & Install

Requirements: Go 1.21+, `protoc` only if you plan to modify the API.

```bash
git clone <repo>
cd goproc
make build          # produces ./goproc
```

You can run commands straight from the repo (`./goproc …`) or move the binary anywhere on your `$PATH`.

---

## Configuration

All CLI commands accept `--config <path>` which points to a JSON file with daemon settings:

```json
{
  "liveness_interval": "15s",
  "last_seen_interval": "45s"
}
```

Durations use Go syntax. Missing keys fall back to sensible defaults.

Environment overrides are available and win over file values:

| Variable                   | Description                                |
|---------------------------|--------------------------------------------|
| `GOPROC_LIVENESS_INTERVAL` | Period between background `kill(pid,0)` probes. |
| `GOPROC_LAST_SEEN_INTERVAL` | Minimum interval for bumping `LastSeen`.        |

Runtime files live in `${GOPROC_RUNTIME_DIR:-$XDG_RUNTIME_DIR}/goproc.sock` on Linux, or `/tmp/goproc-<uid>.sock` on other UNIX systems. The same directory also stores the PID file and snapshot.

---

## CLI Commands

Below is a detailed reference. Unless stated otherwise, every command talks to the running daemon and inherits `--config`.

### `goproc daemon`
Starts the daemon in the foreground; press `Ctrl+C` to stop.

Flags:
- `--force, -f`: stop an existing daemon first (sends `SIGTERM`, falls back to `SIGKILL`).

On start the daemon:
1. Ensures the runtime directory exists.
2. Binds the UNIX socket (removing stale sockets if the daemon isn’t alive).
3. Loads the previous snapshot, if any.
4. Begins liveness probing in the background.

### `goproc ping`
Lightweight health check. Fails immediately if the socket is missing, otherwise performs a gRPC Ping and prints `pong`.

Flag:
- `--timeout, -t <seconds>` (default `2`) — overall RPC timeout.

### `goproc add <pid>`
Registers an existing PID with optional tags/groups and an optional unique name.

Behavior:
- Validates that the daemon is reachable.
- Converts `<pid>` to an integer and rejects non-positive values.
- Issues `kill(pid, 0)` to make sure the process exists and is accessible.
- If the PID is already tracked, the daemon returns `AlreadyExists` with the previous ID.
- On success prints `Process <pid> registered with id <id>`.

Flags:
- `--tag <name>` (repeatable) — attaches labels to the entry.
- `--group <name>` (repeatable) — group membership for bulk queries later.
- `--name <value>` — assigns a unique name; rejected if another entry already uses it.

### `goproc run -- <command> [args...]`
Convenience wrapper around `add` that launches a new process, keeps its stdio attached, and registers the freshly spawned PID with the daemon right away.

Behavior:
- Requires the daemon to be running; the CLI starts the child locally, then calls `Add`.
- Prints the PID and registry ID once registration succeeds, then exits while the child keeps running.
- Records the real command line in the registry so it shows up in `list --search` results.

Flags mirror `add` plus a timeout for the daemon RPC:
- `--tag`, `--group`, `--name` — same semantics as `add`.
- `--timeout <seconds>` (default `3`) — fail if the daemon cannot be reached fast enough.

### `goproc list`
Shows the registry, one line per process:

```
[id=12] pid=4242 name=db-reader alive=true cmd=pid:4242 tags=[db,read] groups=[prod]
```

Filters can be combined:

| Flag            | Meaning |
|-----------------|---------|
| `--tag <value>` | Match entries that have **any** of the provided tags. |
| `--tag-all <value>` | Require entries to have **all** provided tags. |
| `--group <value>` | Match entries in any of the provided groups. |
| `--group-all <value>` | Require entries to be in all groups. |
| `--pid <pid>` | Filter by OS PID (repeatable). |
| `--id <id>` | Filter by registry ID (repeatable). |
| `--name <value>` | Filter by exact process name (repeatable). |
| `--alive` | Only show entries currently deemed alive. |
| `--search <text>` | Substring match against the stored command. |

When no filters are provided it lists everything.

### `goproc rm`
Deletes entries from the registry using the same selectors as `list`.

Flags:
- `--tag`, `--group`, `--pid`, `--id`, `--name` — selectors identical to `list`; `--name` matches exact unique names.
- `--search <text>` — substring search over the stored command (same as `list --search`).
- `--all` — required if the selectors match more than one entry; prevents accidental mass deletion.
- `--timeout <seconds>` — RPC timeout (default `3`).

Successful removals are echoed back with their ID/PID info.

### `goproc kill`
Terminates processes that match the provided selectors, then removes them from the registry.

Flags:
- `--tag`, `--group`, `--name`, `--id`, `--pid` — same selectors as `list`. Only alive entries are terminated.
- `--all` — acknowledge killing more than one alive match.
- `--timeout <seconds>` — covers the list/kill/remove RPCs (default `5`).

For each process the command prints whether the kill succeeded and whether the registry entry was removed. If no alive process matches, nothing is sent to the daemon.

### `goproc tag <name>`
Lists processes that carry a specific tag and optionally renames that tag across the registry before listing.

Flags:
- `--rename <new>` — atomically rename the tag `name` → `<new>` via the daemon’s `RenameTag` RPC.
- `--timeout <seconds>` — default `3`.

If no process has the tag, the command prints a friendly “not found” message.

### `goproc group <name>`
Analogous to `tag`, but matches `Groups` instead of tags. Supports the same `--rename` and `--timeout` flags.

### `goproc reset`
Dangerous operation that wipes the registry, deletes all snapshots, and resets the monotonic ID counter back to `1`.

Safety checks:
- Requires the daemon to be running.
- Requires `--confirm RESET`; anything else fails immediately.
- Blocks for up to `--timeout <seconds>` (default `5`).

Use this sparingly—every tracked process is forgotten after the reset.

---

## Daemon Internals

- **Registry (`internal/registry`)** — thread-safe maps (`byID`, `byPID`, `byName`, `byTag`, `byGroup`). Every mutation persists a JSON snapshot near the socket (unless snapshots are disabled).
- **Liveness ticker** — interval configurable via config/env. Each tick performs `kill(pid, 0)` and updates the `Alive` flag and `LastSeen`.
- **Snapshots** — stored as `goproc.snapshot.json`. On startup the daemon loads the snapshot to reconstruct the registry. The new `reset` command clears the snapshot as well.
- **Process metadata** — monotonic `uint64` IDs, PID, PGID, optional unique name, command string (`pid:<pid>` for now), tags, groups, and timestamps.

---

## Development

- `make build` — compile the CLI.
- `make fmt` — run `go fmt ./...`.
- `make proto` — regenerate gRPC stubs; requires `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc`.

Planned enhancements include richer metadata (CPU/RSS stats), discovery helpers, and more mutating RPCs. Contributions are welcome—open an issue or PR with ideas!
