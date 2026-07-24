# Configuration

Tillandsia uses two configuration files: `tillandsia.yaml` (project config) and
`Procfile` (service definitions). Both live in the root of your project.

## `tillandsia.yaml`

```yaml
# Project name — used as the container name and server directory.
# If not set, defaults to the directory name.
name: my-app

# The domain to serve your app on.
# Caddy will handle TLS via Let's Encrypt (HTTP-01 challenge).
# Create an A record pointing to your VPS IP before deploying.
# If not set, the app is served on the server's IP (no TLS).
domain: my-app.example.com

# The port your web service binds to inside the container.
# Defaults to 8080 if not set.
port: 8080

# Environment variables (plain text).
# For secrets, use `tillandsia env set` (they're stored in an env file on the server).
env:
  NODE_ENV: production
  LOG_LEVEL: info

# Docker build configuration.
build:
  # Dockerfile path (defaults to ./Dockerfile)
  dockerfile: Dockerfile
  # Build context (defaults to .)
  context: .
  # Build args passed to docker build
  args:
    RUNTIME_VERSION: 20

# Litestream / SQLite replication configuration.
# Credentials are stored in the server config (configured during server setup).
# See: docs/server.md
litestream:
  # S3-compatible bucket URL (required for persistence)
  # Supported: S3, R2 (Cloudflare), B2 (Backblaze)
  url: s3://my-bucket/tillandsia/my-app
  # Region for the bucket
  region: us-east-1

# Authentication middleware (optional).
# When configured, Caddy handles OAuth before requests reach your app.
# Your app receives identity headers instead of needing auth code.
auth:
  providers:
    - github
    - google
  allowed:
    - "*@mycompany.com"
```

## `Procfile`

Declares the services that run inside your container. Format from Heroku:

```
web: node server.js
worker: node worker.js
cron: node cron.js
```

Service types:
- **web** — Receives HTTP traffic via Caddy. Must bind to `$PORT` (or whatever port
  you configured in `tillandsia.yaml`).
- **worker** — Background worker. No external traffic. Shares filesystem and SQLite.
- **cron** — Scheduled or long-lived background process. No external traffic.

## Full Defaults

If you only have a `Procfile` and no `tillandsia.yaml`, Tillandsia uses:

```yaml
name: <directory-name>
port: 8080
build:
  dockerfile: Dockerfile
  context: .
```

## `tillandsia.yaml` Discovery

Tillandsia looks for the config file in this order:
1. `tillandsia.yaml` in the current directory
2. `tillandsia.yml` in the current directory
3. Falls back to defaults (no config file needed for simple projects)