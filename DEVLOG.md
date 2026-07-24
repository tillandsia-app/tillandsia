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

### Total project state

```
tillandsia/
├── cmd/
│   ├── tillandsia/main.go
│   └── tillandsia-init/main.go (stub)
├── internal/
│   ├── cli/
│   │   ├── root.go          # cobra root, --json, --yes, --debug flags
│   │   ├── init.go          # project scaffolding
│   │   ├── help.go          # embedded docs
│   │   └── server.go        # NEW: server management commands
│   ├── server/
│   │   └── setup.go         # NEW: Docker install, health checks
│   ├── ssh/
│   │   ├── ssh.go           # NEW: SSH connection, commands, transfer
│   │   └── ssh_test.go      # NEW: key generation tests
│   ├── build/
│   │   ├── runtime.go       # Dockerfile generator registry
│   │   └── runtime_test.go
│   ├── config/
│   │   └── config.go        # YAML config parsing, server config
│   └── types/
│       └── types.go         # shared types
├── dev-docs/
│   ├── implementation.md
│   └── notes/...
├── docs/                    # embedded documentation
├── go.mod / go.sum
├── Makefile
├── .goreleaser.yaml
└── DEVLOG.md               # NEW: this file
```