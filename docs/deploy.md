# Deploy

The deploy pipeline is the core of Tillandsia — a single command that takes your
application from source code to a running, TLS-protected, database-backed service
on your VPS.

## `tillandsia deploy`

```bash
# From your project directory:
tillandsia deploy

# Deploy to a specific server:
tillandsia deploy --server my-vps

# Dry run (show what would happen):
tillandsia deploy --dry-run

# JSON output for agents:
tillandsia deploy --json
```

## What Happens

1. **Read config** — Loads `tillandsia.yaml` and `Procfile` from the project root.
2. **Select server** — Uses the default server, or the one specified with `--server`.
3. **Build image** — Runs `docker build` with your Dockerfile and build context.
4. **Wrap image** — Creates a new image that layers `tillandsia-init`, `caddy`, and
   `litestream` on top of your app image, with `ENTRYPOINT tillandsia-init`.
5. **Transfer image** — Pipes the image to the server via `docker save | ssh docker load`.
6. **Prepare server** — Stops the old container (if any), writes the env file.
7. **Start container** — Runs the new container with the init system as PID 1.
8. **Health check** — Waits for the management API to report healthy.
9. **Report success** — Prints the URL where the app is reachable.

## Deploy Modes

### Fresh deploy (no previous version)

The container starts from scratch. If Litestream is configured, the database is
restored from the latest snapshot before the app starts. If no snapshot exists
(first deploy), a fresh SQLite database is created.

### Rolling update (same container, no downtime)

When a new version is deployed:
1. The new container starts alongside the existing one.
2. The new container signals the old one via the management port.
3. The old container stops accepting requests, flushes Litestream, and exits.
4. Traffic is routed to the new container.

This requires the management port to be accessible between containers.
It's enabled by default when both containers are on the same Docker network.

## `tillandsia status`

```bash
# Show current deploy state:
tillandsia status

# JSON output:
tillandsia status --json
```

Returns:
- Whether the app is deployed
- Container health
- Uptime
- Current version / image hash
- Litestream replication lag
- Environment variable count

## `tillandsia logs`

```bash
# Tail logs from the running container:
tillandsia logs

# Last 100 lines:
tillandsia logs --tail 100

# Follow mode (default):
tillandsia logs --follow

# JSON output:
tillandsia logs --json
```

## Validation

Before deploying, Tillandsia validates:
- The server is reachable via SSH
- Docker is installed on the server
- The Dockerfile exists and builds
- The Procfile is valid
- Env vars referenced in `tillandsia.yaml` exist in the server's env file
- The domain (if configured) resolves to the server's IP

## Rollback

```bash
# Deploy the previous version:
tillandsia deploy --rollback
```

This re-deploys the previous image (which is still cached on the server).