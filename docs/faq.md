# FAQ

## General

### What is Tillandsia?

A self-hosted deployment platform. You run a CLI on your machine, point it at a VPS,
and deploy applications with a single command. TLS, SQLite backups, and process
supervision are all handled automatically.

### Is it ready?

Not yet. This is an open source project under active development.

### How is this different from Heroku?

Heroku is a managed service. Tillandsia runs on your own VPS. You own the
infrastructure, there's no platform fee, and you're not locked into a proprietary
runtime.

### How is this different from Dokku?

Dokku is a similar concept but heavier — it runs a full PaaS stack on the server
(Herokuish buildpacks, its own proxy, etc.). Tillandsia is lighter: a single Go
binary for the init system, Caddy for the proxy, and Litestream for backups.
You write your own Dockerfile (or `tillandsia init` generates one).

### How is this different from CapRover / Coolify / Piku?

Similar goals, different design. Tillandsia is purpose-built for the init system
pattern (single container, multiple cooperating processes, Litestream for durable
SQLite). It's designed from the ground up to be agent-friendly (JSON output,
non-interactive mode, embedded docs).

## Technical

### What databases does Tillandsia support?

SQLite is the default and primary database. Litestream provides continuous backup
to S3-compatible object storage. You can also run any other database inside the
container or connect to an external database via environment variables.

### Can I use PostgreSQL or MySQL?

Yes. Set `DATABASE_URL` as an environment variable and connect to an external
database. Tillandsia doesn't manage external databases — you run them wherever
you want.

### How does Litestream work?

Litestream continuously streams SQLite write-ahead log (WAL) changes to object
storage. On startup, the latest snapshot is restored before the app boots. This
turns SQLite into a durable production database with point-in-time recovery.

### What object storage is supported?

Any S3-compatible storage: AWS S3, Cloudflare R2, Backblaze B2, MinIO, DigitalOcean
Spaces, etc.

### Does Tillandsia support multiple servers?

Yes. You can add multiple servers and deploy to different ones for staging,
production, or geographic distribution.

### How do I scale my app?

SQLite in WAL mode handles hundreds of thousands of reads per second on modest
hardware. If you outgrow it, you can migrate to a client-server database. For
horizontal scaling, you can run multiple containers behind a load balancer.

### What about zero-downtime deploys?

Supported. The new container starts alongside the old one, coordinates via the
management port, and takes over traffic when ready. See `docs/concepts.md`.

## Operations

### What VPS providers are supported?

Any Linux VPS that supports Docker. DigitalOcean, Linode, Vultr, Hetzner, AWS EC2,
and any other provider. As long as you can SSH in and run Docker, it works.

### What are the minimum server requirements?

- 1 CPU core, 512 MB RAM, 10 GB disk
- Ubuntu 22.04+ or Debian 11+
- Ports 22, 80, and 443 open

### How do backups work?

Litestream continuously backs up your SQLite database to S3-compatible object
storage. To restore, just deploy to a new server — the init system restores the
latest snapshot on startup.

### How do I monitor my app?

`tillandsia status` shows health, uptime, and Litestream replication lag.
`tillandsia logs` streams application logs. The management API (`/mgmt/health`)
provides programmatic health checks.

### How do I update Tillandsia?

Download the latest binary from the releases page. The init system is bundled
into your application image at deploy time, so updating the CLI means the next
deploy will use the new init system.

## Licensing

### Is Tillandsia open source?

Yes. The core binary, init system, and CLI are all open source. See the license
file for details.

### What's the business model?

Managed hosting (the "Starter" and "Pro" tiers), a hosted registry for faster
deploys, and enterprise features like custom DNS zones and team management.

### Can I use Tillandsia commercially?

Yes. The open source core is free to use on your own infrastructure.