# Architecture

Tillandsia is two Go binaries that work together to deploy applications on a VPS.

## The Two Binaries

### `tillandsia` (CLI)

Runs on your local machine. Manages servers, builds images, deploys apps, streams logs,
and manages environment variables. Communicates with the VPS over SSH.

### `tillandsia-init` (Init System)

Runs as PID 1 inside the application container on the VPS. Starts, supervises, and
coordinates all processes: the user's application services, Litestream (SQLite
replication), and Caddy (reverse proxy / TLS termination).

## Deploy Flow

```
┌─ Your Machine ─────────────────────────────────┐
│  tillandsia deploy                              │
│    ├─ Read tillandsia.yaml / Procfile           │
│    ├─ Build Docker image                        │
│    ├─ Wrap with init system                     │
│    ├─ docker save | ssh docker load             │
│    └─ SSH: run the container                    │
│                                                 │
│  tillandsia logs                                │
│    └─ SSH: docker logs -f                       │
│                                                 │
│  tillandsia env set                             │
│    └─ SSH: write env file, restart container    │
│                                                 │
│  tillandsia server setup                        │
│    └─ SSH: install Docker, prep directories     │
└─────────────────────────────────────────────────┘

┌─ VPS ───────────────────────────────────────────┐
│  Docker container:                               │
│  ┌──────────────────────────────────────────┐    │
│  │  tillandsia-init (PID 1)                 │    │
│  │    ├─ Restore DB from Litestream         │    │
│  │    ├─ Start Litestream replication       │    │
│  │    ├─ Start user services                │    │
│  │    ├─ Generate Caddy config              │    │
│  │    ├─ Start Caddy (after health checks)  │    │
│  │    └─ Management API (:8080/mgmt/*)      │    │
│  │                                          │    │
│  │  ├─ Caddy (TLS, reverse proxy)           │    │
│  │  ├─ Litestream (SQLite → S3/R2/B2)      │    │
│  │  └─ User app (web, worker, cron)         │    │
│  └──────────────────────────────────────────┘    │
│  Persistent volume: /data/ (SQLite DB + config)  │
└──────────────────────────────────────────────────┘
```

## Container Layout

The user's application image is used as a base. At deploy time, the CLI wraps it:

```dockerfile
FROM user-built-image
COPY tillandsia-init /usr/local/bin/
COPY caddy /usr/local/bin/
COPY litestream /usr/local/bin/
ENTRYPOINT ["tillandsia-init"]
```

This wrapper is transparent — users can inspect exactly what runs in their container.

## Init System Startup Sequence

1. **Litestream restore** (synchronous, blocking) — the app never starts against a stale
   database. If no snapshot exists (first deploy), this step is a no-op.
2. **Litestream replication** begins in the background.
3. **All app services start** — SQLite is available immediately via the local filesystem.
4. **Caddy starts routing** to web services, but only after health checks pass.

## Service Types

The Procfile defines service types that the init system understands:

- **web** — Gets Caddy routing in front of it. Must bind to `$PORT`.
- **worker** — No external traffic. Shares filesystem and SQLite with web services.
- **cron** — No external traffic. Runs on a schedule or as a long-lived process.

All services share the same filesystem, network namespace, and SQLite database. IPC
is as simple as reading and writing to the same tables, or communicating over localhost.

## Management API

The init system exposes a management port (internal to the container) for health checks
and orchestration:

```
GET  /mgmt/health       → liveness and readiness state
GET  /mgmt/identity     → instance certificate info
POST /mgmt/shutdown     → initiate graceful shutdown sequence
```

## Image Transfer

For the free tier, images are transferred via SSH pipe:

```bash
docker save <image> | ssh <server> docker load
```

No registry required. This is the default. A container registry is a paid upgrade
for faster incremental deploys.