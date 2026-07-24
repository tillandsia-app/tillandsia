# Tillandsia

> *Deploy anywhere. No roots required.*

Tillandsia is an open source, self-hostable deployment platform designed for the era of AI-generated applications. Like the air plant it's named after, your app should be able to grow anywhere — on a $6 VPS, your company's Kubernetes cluster, or a managed cloud account — without needing a complex infrastructure substrate to survive.

The core premise: the app you built in one conversation should deploy in one command.

```bash
tillandsia deploy
```

---

## Why This Exists

The "vibe coding" wave is producing enormous numbers of small, capable applications with no good home. Heroku is dead. Railway and Render are excellent but you don't own the infrastructure. Kubernetes is far too much for a weekend project. There is no tool that treats SQLite as a first-class production database, handles deployment as a single command, and lets you own the infrastructure it runs on.

Tillandsia fills that gap, with a philosophy that matches the way small apps are built today: radical simplicity, sane defaults, and an escape hatch for everything.

---

## Product Tiers

| | BYOS | Starter | Pro | Enterprise |
|---|---|---|---|---|
| Infrastructure | Yours | Ours (flat rate) | Ours (flat rate) | Yours or ours |
| Platform fee | Free | Free | $5/seat | Contract |
| Subdomains | 1 | 1 | Unlimited | Unlimited |
| Secret management | ✓ | ✓ | ✓ | ✓ |
| Basic OAuth (Google, GitHub, Auth0) | ✓ | ✓ | ✓ | ✓ |
| Basic logging & monitoring | ✓ | ✓ | ✓ | ✓ |
| Log retention | 7 days | 7 days | 30 days | 90 days |
| Backup management | — | — | ✓ | ✓ |
| Team sharing | — | — | ✓ | ✓ |
| Independent service scaling | — | — | ✓ | ✓ |
| Custom OAuth / OIDC | — | — | — | ✓ |
| Deployment policies | — | — | — | ✓ |
| Bring your own infrastructure | — | — | — | ✓ |
| Custom DNS zone | — | — | — | ✓ |

**Pricing philosophy:** secrets and basic auth are security primitives, not premium features. The free tier is genuinely complete and useful. Paid tiers add operational maturity and team features, not artificial capability gates. Infrastructure is priced at cost plus a flat margin, sold externally as fixed tier prices.

---

## Technical Design

### The Init System

Every Tillandsia deployment runs a single container containing the user's application plus three additional binaries managed by a lightweight Go init system:

- **Caddy** — reverse proxy, TLS termination, automatic Let's Encrypt
- **Litestream** — continuous SQLite replication to object storage
- **tillandsia-init** — the glue that starts, supervises, and coordinates everything

The init system starts processes in a defined order and supervises all of them. A Procfile defines the user's services:

```
web: node server.js
worker: node worker.js
cron: node cron.js
```

The init system understands service types. `web` services get Caddy routing in front of them. `worker` and `cron` services share the filesystem and Litestream replication but receive no external traffic. All services share the same filesystem, network namespace, and SQLite database — IPC is as simple as reading and writing to the same tables, or communicating over localhost.

**Startup sequence:**
1. Litestream restore (synchronous, blocking — app never starts against a stale database)
2. Litestream replication begins (background)
3. All app services start (SQLite is available immediately)
4. Caddy starts routing to web services (only after health checks pass)

**Caddy configuration** is generated from environment variables at startup:

```
your-app.tillandsia.app {
    reverse_proxy localhost:$APP_PORT
}
```

TLS is fully automatic. The user configures a domain; Caddy handles issuance and renewal.

The user's app container is treated as a base image. At deploy time the CLI wraps it:

```dockerfile
FROM user-built-image
COPY tillandsia-init /usr/local/bin/
COPY caddy /usr/local/bin/
COPY litestream /usr/local/bin/
ENTRYPOINT ["tillandsia-init"]
```

This is transparent and auditable. Users can inspect exactly what's running in their container.

---

### SQLite as the Default Database

SQLite in WAL mode is the right database for the vast majority of small applications. It requires zero operational overhead, ships as part of the deployment, and eliminates the network round trips that slow down client-server databases. All services in the container open the same SQLite file directly — no connection strings, no connection pooling, no network database calls between services.

Litestream provides continuous replication to object storage (S3, R2, B2, etc.), turning SQLite into a durable production database. On startup, the latest snapshot is restored before the app boots. On every write, changes are streamed to object storage in near-realtime.

**Scaling ceiling:** SQLite in WAL mode handles hundreds of thousands of reads per second on modest hardware. Write throughput is the constraint, but most small applications are overwhelmingly read-heavy. The realistic ceiling before a database migration is warranted is well past the point where an app has become a real business with real engineering resources.

When users genuinely need independent service scaling, the single container becomes multiple containers on a shared Docker network with a shared volume. From the app's perspective nothing changes — SQLite is at the same path, localhost calls still work, the Procfile is unchanged. The abstraction holds through the upgrade.

---

### Service Identity and mTLS

Every application in Tillandsia receives a root identity expressed as a certificate. Every running instance of that application receives a short-lived instance certificate chained to the app's root. This follows the SPIFFE standard (Secure Production Identity Framework For Everyone), giving Tillandsia interoperability with existing enterprise identity infrastructure.

This enables several things:

**Zero-downtime deploys via cooperative handoff.** When a new version is deployed, the new container (B) starts alongside the existing one (A). Using mTLS on a management port, the containers coordinate explicitly:

1. B comes up and establishes its identity
2. B contacts A on the management port, notifying it to begin shutdown
3. A stops accepting new requests and flushes its database to Litestream
4. A notifies B that shutdown is complete
5. B syncs any remaining Litestream changes and starts the application
6. Caddy begins routing traffic to B

Downtime is bounded by app startup time. The SQLite lock handoff is cooperative rather than forced, eliminating the race condition.

**Secure service-to-service communication.** Multiple Tillandsia apps can call each other with verified identity out of the box. No shared secrets, no API keys to rotate or leak, no configuration required. All internal traffic is mutually authenticated and encrypted.

**Local development as a mesh participant.** The CLI can issue a short-lived certificate to the developer's local machine, chained to the app's root identity. The local environment becomes a genuine instance in the service mesh — it can call staging services with verified identity and receive callbacks over a tunnel. "Works on my machine" failures caused by environmental differences are eliminated.

**The management port** is implemented entirely in the init binary and requires no changes to user code:

```
POST /mgmt/shutdown       → initiate graceful shutdown sequence
GET  /mgmt/health         → liveness and readiness state
GET  /mgmt/sync-status    → litestream replication lag
POST /mgmt/checkpoint     → force a litestream sync
GET  /mgmt/identity       → return this instance's certificate info
```

---

### DNS and TLS

Tillandsia hosts the `*.tillandsia.app` DNS zone. On first deploy, the CLI registers a subdomain (e.g. `velvet-moth.tillandsia.app`) pointing to the user's server. Because Tillandsia controls the zone, DNS-01 ACME challenges are handled centrally — TLS certificates are issued and renewed automatically with no action required from the user.

Custom domains work entirely on the user's server. The user creates a CNAME to their Tillandsia subdomain, and the init system handles Let's Encrypt HTTP-01 challenges locally. Tillandsia's hosted infrastructure is not involved. Custom domain support is free at all tiers.

Enterprise customers have two options for private DNS zones:

**Zone delegation (preferred).** The enterprise creates an NS record delegating a subdomain (e.g. `apps.mycompany.com`) to Tillandsia's nameservers. Tillandsia creates A records dynamically as apps deploy and handles all cert issuance via DNS-01. Nothing else is required.

**Wildcard certificate.** For enterprises whose security policy prohibits zone delegation, Tillandsia can use their DNS provider's API for record management and accept a wildcard certificate for the zone. Suited for internal PKI or longer-lived certs from a commercial CA.

Both paths work fully behind a VPN or corporate firewall. DNS-01 challenges never require Let's Encrypt to reach the server. Tillandsia requires only outbound HTTPS (port 443) from the server — no inbound firewall rules are needed.

---

### Application-Layer Authentication

OAuth for common providers is built into the proxy layer, not the application. Users configure auth in their Tillandsia config:

```yaml
auth:
  providers: [github, google]
  allowed:
    - "*@mycompany.com"
```

The init system injects an auth middleware into Caddy. The OAuth dance, session management, and token validation are handled transparently. The user's app receives authenticated requests with identity headers already set:

```
X-User-Email: alice@mycompany.com
X-User-Name: Alice
```

No auth code required in the application. Built-in providers (Google, GitHub, Microsoft, Auth0) are available at all tiers. Custom OAuth/OIDC providers for enterprise identity systems (Okta, Ping, internal IdPs) are available at the enterprise tier.

---

### Bring Your Own Infrastructure (Enterprise)

Enterprise customers with existing compute infrastructure can use Tillandsia as the developer-facing API over their own stack. Tillandsia defines a simple infrastructure provider interface:

```
POST /provision      → here's a service spec, give me somewhere to run it
POST /deprovision    → tear this down
GET  /status         → what's the state of this thing
```

The enterprise's platform team implements these webhooks however they choose — Terraform, Pulumi, internal tooling, Kubernetes operators. Tillandsia calls the webhooks; what happens on the other side is entirely under the enterprise's control.

The alternative managed path — providing Tillandsia with cloud credentials and having it provision infrastructure directly — is also supported for enterprises that prefer it.

In both cases the developer experience is unchanged. The Procfile is the same. The CLI commands are the same. Tillandsia becomes the developer-facing API for infrastructure the enterprise already owns and operates.

---

### Open Source Model

The core binary, init system, DNS provisioner, and infrastructure provider interface are all open source. The hosted control plane — team management, deploy dashboard, centralized logging, backup management UI, billing — is the commercial layer built on top.

The free tier is genuinely complete. A developer can build and deploy a real production application, with TLS, secrets management, OAuth, automatic database backups, and a public URL, without spending a dollar. The paid tier adds operational maturity, team features, and managed infrastructure — not capabilities that should have been free.
