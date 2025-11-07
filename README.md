# goproc

Small UNIX process watcher with a local daemon and CLI client connected over gRPC.

## What you can do right now

- **Run the daemon:** `./goproc daemon` starts a background service bound to a per-user UNIX socket (see `internal/daemon`).
- **Health check:** `./goproc ping` reports `pong` if the daemon is reachable.
- **Register existing processes:** `./goproc add <pid> [--tag foo --group bar]` sends an `Add` RPC. The daemon performs a `kill(pid, 0)` probe up front, so you cannot register non-existent or inaccessible processes.
- **List tracked processes:** `./goproc list` prints the registry. Filters are available via `--tag`, `--tag-all`, `--group`, `--group-all`, `--pid`, `--id`, `--alive`, and `--search`.

Behind the scenes the daemon maintains a thread-safe registry with secondary indexes (IDs, PIDs, tags, groups) plus JSON snapshots on disk. A background ticker periodically re-validates liveness (`kill(pid, 0)`) so the `Alive` flag and `LastSeen` timestamps stay fresh.

## Building & Running

```bash
make build          # builds ./goproc
./goproc daemon     # start the daemon (Ctrl+C to stop)
./goproc add 1234 --tag db --group prod
./goproc list --tag db --alive
```

### Configuration

You can point the CLI at a JSON config file:

```bash
./goproc --config config.example.json daemon
```

Supported keys (durations use Go syntax, e.g. `30s`, `2m`):

```json
{
  "liveness_interval": "15s",
  "last_seen_interval": "45s"
}
```

Environment overrides are also available:

- `GOPROC_LIVENESS_INTERVAL`
- `GOPROC_LAST_SEEN_INTERVAL`

Both override file/default values if set to valid durations (`> 0`).

## CLI Overview

| Command         | Description                                                      |
|-----------------|------------------------------------------------------------------|
| `goproc daemon` | Starts the daemon, writes PID & socket files, handles `--force`. |
| `goproc ping`   | Fast health check (`PING` RPC).                                  |
| `goproc add`    | Registers an existing PID; refuses missing/inaccessible PIDs.    |
| `goproc list`   | Displays registry entries with filters (tags/groups/etc).        |

All commands accept `--config <path>` to reuse the same configuration file.

## What’s next?

- Extend the gRPC API with mutate/tag RPCs and richer CLI commands (`tag`, `group`, `kill`, etc.).
- Surface metrics (CPU, RSS, IO) once the watcher gathers them.
- Add process discovery helpers (e.g., register by executable name or pattern).

Contributions and ideas are welcome—open an issue or a PR. Happy hacking!
