# DEVLOG

## 2026-07-24 — Phases 2 & 3

### Phase 2: SSH Module

Implemented the SSH connection manager in `internal/ssh/ssh.go`:

- **Key management:** Tillandsia generates and manages its own ed25519 keypair in
  `~/.config/tillandsia/ssh/`. The `EnsureKey()` function creates the keypair on
  first call (idempotent thereafter). `PublicKey()` returns the public key for
  injecting into VPS instances during provisioning.
- **Key search order during `Connect`:**
  1. Explicit `keyPath` from server config (if set)
  2. Tillandsia-managed keys (`~/.config/tillandsia/ssh/id_ed25519`)
  3. User's standard SSH keys (`~/.ssh/id_ed25519`, `id_rsa`, etc.)
  4. Password prompt as fallback
- **Remote execution:** `Run()` captures stdout/stderr/exit code; `RunStream()`
  provides real-time streaming output for `tillandsia logs`.
- **File transfer:** `Transfer()` pipes data through SSH stdin to `cat > dest`
  on the remote side (for `docker save | ssh docker load`).
- **Connection testing:** `Test()` verifies SSH connectivity and Docker availability.
- **Portability note:** `dev-docs/notes/key-portability.md` — future plan for syncing
  server configs to S3/Spaces bucket for machine migration.

**Dependencies added:** `golang.org/x/crypto`, `golang.org/x/term`

### Phase 3: Server Management

Implemented all `tillandsia server` subcommands in `internal/cli/server.go` and
`internal/server/setup.go`:

- `server add <name> --host <host>` — registers server in `~/.config/tillandsia/servers.yaml`,
  validates SSH connectivity before saving. Supports `--user`, `--key`, `--port`, `--default`.
- `server setup <name>` — SSH into server, installs Docker via `get.docker.com` (official
  script, handles all distros), creates `/var/lib/tillandsia/`, verifies with `hello-world`.
  Confirmation prompt (skippable with `--yes`).
- `server ls` — lists registered servers with default indicator, `--json` output.
- `server inspect <name>` — shows server details and tests connectivity.
- `server rm <name>` — removes from config with confirmation prompt.
- `server default <name>` — sets default deploy target.

All commands support `--json` and `--yes` where applicable.

## 2026-07-24 — Phase 4: Init System

### Module rename

`github.com/tillandsia/tillandsia` → `github.com/tillandsia-app/tillandsia` (correcting
the GitHub org). All imports across the codebase updated.

### New `internal/supervisor/` package

Replaces the old `internal/init/` stub. Runs as PID 1 inside deployed containers.

**`supervisor.go`** — Process lifecycle manager:
- Starts Litestream → user services → Caddy in strict order
- Monitors each process in a goroutine, restarts on crash (configurable retry limit, default 3)
- Streams stdout/stderr per process with `[name]` prefix
- On SIGTERM/SIGINT: stops Caddy first (drain connections), sends SIGTERM to services with
  15s timeout, then kill, flushes Litestream WAL, exits

**`caddy.go`** — Caddy embedded as Go library (`github.com/caddyserver/caddy/v2`):
- Builds a JSON config with reverse proxy to `localhost:$PORT` at `:80` (or domain + `:443`)
- `caddy.Run()` / `caddy.Stop()` — no binary needed
- Supports custom Caddyfile override via `TILLANDSIA_CADDYFILE` env var

**`litestream.go`** — Litestream embedded as Go library (`github.com/benbjohnson/litestream`):
- `DB.Open()` + `Replica.Start()` for continuous replication — no binary needed
- `Restore()` with timeout, skips if DB already exists
- `Flush()` calls `SyncAndWait()` + `Snapshot()` on shutdown
- Config from env vars (`LITESTREAM_URL`, `LITESTREAM_ACCESS_KEY_ID`, etc.)

**`health.go`** — HTTP health checker:
- Polls `http://localhost:$PORT/$path` at 2s intervals
- 60s timeout, returns on first 2xx–4xx response
- `HealthStatus` struct with liveness, readiness, per-service status, Caddy state, Litestream lag

**`mgmt.go`** — Management HTTP API on `:8080`:
- `GET /mgmt/health` — full health status as JSON
- `POST /mgmt/shutdown` — triggers graceful shutdown
- `GET /mgmt/identity` — hostname, start time, uptime

### Updated `cmd/tillandsia-init/main.go`

Real entry point that reads config from `tillandsia.yaml` (in `/app` or `.`) merged with
environment variables. Supports `TILLANDSIA_SERVICES`, `TILLANDSIA_ENV`, `TILLANDSIA_DOMAIN`,
`LITESTREAM_*`, `PORT`, etc.

### Total project state

```
tillandsia/
├── cmd/
│   ├── tillandsia/main.go
│   └── tillandsia-init/main.go  # REAL: init system entry point
├── internal/
│   ├── cli/
│   │   ├── root.go          # cobra root, --json, --yes, --debug flags
│   │   ├── init.go          # project scaffolding
│   │   ├── help.go          # embedded docs
│   │   └── server.go        # server management commands
│   ├── server/
│   │   └── setup.go         # Docker install, health checks
│   ├── ssh/
│   │   ├── ssh.go           # SSH connection, commands, transfer
│   │   └── ssh_test.go      # key generation tests
│   ├── build/
│   │   ├── runtime.go       # Dockerfile generator registry
│   │   └── runtime_test.go
│   ├── config/
│   │   └── config.go        # YAML config parsing, server config
│   ├── supervisor/           # NEW: init system (PID 1 in container)
│   │   ├── supervisor.go    # process lifecycle, restart, signal handling
│   │   ├── caddy.go         # embedded Caddy reverse proxy
│   │   ├── litestream.go    # embedded Litestream replication
│   │   ├── health.go        # health checker
│   │   └── mgmt.go          # management HTTP API
│   └── types/
│       └── types.go         # shared types
├── dev-docs/
│   ├── implementation.md
│   └── notes/...
├── docs/                    # embedded documentation
├── go.mod / go.sum
├── Makefile
├── .goreleaser.yaml
└── DEVLOG.md
```