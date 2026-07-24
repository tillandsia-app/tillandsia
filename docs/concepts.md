# Concepts

## The Init System

`tillandsia-init` is a lightweight Go binary that runs as PID 1 inside the
application container. It is the orchestrator that manages all other processes:

- Starts processes in a defined order
- Supervises them (restarts on crash)
- Handles graceful shutdown
- Coordinates zero-downtime deploys

The init system is not a full init daemon like systemd. It's a purpose-built
supervisor for the specific pattern of one container running multiple cooperating
processes.

## Procfile

The Procfile is a Heroku-inspired format for declaring what runs inside your
container. Each line defines a service type and the command to run:

```
web: node server.js
worker: node worker.js
```

Service types:
- **web** — Receives HTTP traffic. The init system tells Caddy to route to it.
- **worker** — Background processing. No external traffic.
- **cron** — Scheduled or persistent background tasks.

All services share the same filesystem, network namespace, and SQLite database.
They communicate by reading/writing the same database or over localhost.

## SQLite as the Default Database

SQLite in WAL mode is the right database for the vast majority of small
applications. It requires zero operational overhead, ships as part of the
deployment, and eliminates network round trips. All services in the container
open the same SQLite file directly — no connection strings, no connection pooling,
no network calls between services.

**Scaling ceiling:** SQLite in WAL mode handles hundreds of thousands of reads
per second on modest hardware. Write throughput is the constraint, but most small
applications are overwhelmingly read-heavy. The realistic ceiling is well past the
point where an app has become a real business with real engineering resources.

## Litestream

Litestream provides continuous SQLite replication to object storage (S3, R2, B2).
This turns SQLite into a durable production database:

- **On startup:** The latest snapshot is restored before the app boots.
- **On every write:** Changes are streamed to object storage in near-realtime.
- **On disaster:** A new container can restore from the latest snapshot anywhere.

### Startup sequence

1. Litestream restore (blocking — app never starts against a stale database)
2. Litestream replication begins (background)
3. App services start
4. Caddy starts routing (after health checks pass)

## Caddy

Caddy is a reverse proxy that handles TLS termination and automatic Let's Encrypt
certificate issuance. The init system generates a minimal Caddy config from
environment variables:

```
my-app.example.com {
    reverse_proxy localhost:8080
}
```

Caddy handles:
- HTTP-01 ACME challenges (certificate issuance and renewal)
- TLS termination
- Request routing to web services
- Optional OAuth authentication middleware

## The Container

Every Tillandsia deployment runs a single Docker container containing:
- The user's application
- `tillandsia-init` (the init system)
- `caddy` (reverse proxy)
- `litestream` (SQLite replication)

The user's app image is used as a base. At deploy time, the CLI layers the three
additional binaries on top and sets `ENTRYPOINT ["tillandsia-init"]`.

## Image Transfer

For the free tier, images are transferred via SSH pipe — no registry required:

```bash
docker save <image> | ssh <server> docker load
```

This is simple and secure, but can be slow for large images. A container registry
is a paid upgrade for faster incremental deploys.

## The Management Port

The init system exposes a management HTTP API on port 8080 (internal to the
container, not exposed to the internet):

```
GET  /mgmt/health       → liveness and readiness
POST /mgmt/shutdown     → initiate graceful shutdown
GET  /mgmt/identity     → instance certificate info
```

This is used for:
- Health checks during deploy
- Zero-downtime handoff between old and new containers
- Monitoring and observability

## Zero-Downtime Deploys

When a new version is deployed, the new container starts alongside the existing
one. Using the management port, the containers coordinate:

1. New container starts and establishes its identity.
2. New container contacts the old container on the management port.
3. Old container stops accepting requests, flushes Litestream, and exits.
4. New container syncs any remaining Litestream changes and starts the application.
5. Traffic routes to the new container.

Downtime is bounded by app startup time. The SQLite lock handoff is cooperative
rather than forced, eliminating race conditions.

## Authentication Middleware

OAuth is handled at the proxy layer, not the application. When configured in
`tillandsia.yaml`, the init system injects an auth middleware into Caddy. The
OAuth dance, session management, and token validation are handled transparently.
The app receives identity headers:

```
X-User-Email: alice@mycompany.com
X-User-Name: Alice
```

No auth code required in the application.