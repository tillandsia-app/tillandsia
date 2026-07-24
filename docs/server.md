# Server Management

A server is a VPS (or any Linux machine) that runs your Tillandsia applications.
The CLI manages servers through SSH.

## Commands

### Add a server

```bash
tillandsia server add my-vps --host 203.0.113.42 --user root --key ~/.ssh/id_rsa
```

- `--host` — IP address or hostname
- `--user` — SSH user (defaults to root)
- `--key` — Path to SSH private key (defaults to `~/.ssh/id_rsa`)
- `--port` — SSH port (defaults to 22)

Servers are stored in `~/.config/tillandsia/servers.yaml`.

### Setup a server

```bash
tillandsia server setup my-vps
```

This connects to the server via SSH and:
1. Installs Docker (if not already installed)
2. Creates the directory `/var/lib/tillandsia/` for app data
3. Prompts for Litestream credentials (S3-compatible bucket access key and secret)
   — these are stored in the server config and injected into containers
   automatically at deploy time. You never need to manage them manually.
4. Verifies Docker is working

The Litestream credentials are stored in your local server config at
`~/.config/tillandsia/servers.yaml`:

```yaml
servers:
  my-vps:
    host: 203.0.113.42
    user: root
    key: /home/user/.ssh/id_rsa
    port: 22
    litestream:
      access-key-id: AKIA...
      secret-access-key: ...
```

### List servers

```bash
tillandsia server list
tillandsia server ls   # shorthand
```

### Show server details

```bash
tillandsia server inspect my-vps
tillandsia server inspect my-vps --json
```

### Remove a server

```bash
tillandsia server rm my-vps
```

This only removes the server from your local config — it does not touch the VPS.

### Set the default server

```bash
tillandsia server default my-vps
```

The default server is used when you run `tillandsia deploy` without `--server`.

### Provision a server (cloud provider)

```bash
# One command creates VPS + bucket + DNS from a single token:
tillandsia server provision do \
  --name my-app \
  --region nyc1 \
  --size s-1vcpu-1gb \
  --domain my-app.example.com

# Or use an environment variable for the token:
export TILLANDSIA_DO_TOKEN=dop_v1_...
tillandsia server provision do --name my-app
```

This single command does everything:

1. **Creates a VPS** (Droplet, EC2, etc.) with an auto-generated SSH key
2. **Creates an S3-compatible bucket** (Spaces, S3, GCS) for Litestream backups
3. **Generates access keys** for the bucket
4. **Creates a DNS A record** pointing your domain to the VPS IP
5. **Installs Docker** on the VPS and prepares the server
6. **Stores all credentials** in the server config — you never see or manage them

The provider abstraction means every cloud provider works the same way:
- One token in
- Running server + bucket + DNS out
- All intermediate credentials handled internally

Currently supported providers:
- **DigitalOcean** (`do`) — Droplets + Spaces + DNS

To tear everything down:

```bash
tillandsia server rm my-app
```

## Server Requirements

- **OS:** Ubuntu 22.04+ or Debian 11+ (others may work but are not tested)
- **CPU:** 1 core (minimum)
- **RAM:** 512 MB (minimum, 1 GB recommended)
- **Disk:** 10 GB (minimum)
- **Ports:** 22 (SSH), 80 (HTTP), 443 (HTTPS) must be open
- **Docker:** Installed automatically by `tillandsia server setup`

## Server Config File

Servers are stored in `~/.config/tillandsia/servers.yaml`:

```yaml
servers:
  # Manually added server (SSH-only)
  my-vps:
    host: 203.0.113.42
    user: root
    key: /home/user/.ssh/id_rsa
    port: 22
    default: true

  # Provisioned server (cloud provider created everything)
  my-app:
    host: 203.0.113.99
    user: root
    key: /home/user/.ssh/tillandsia_my-app  # auto-generated
    port: 22
    provider: digitalocean
    provider_id: "1234567"                    # droplet ID for deprovisioning
    litestream:
      access-key-id: DO00ABC...
      secret-access-key: ...
      url: s3://my-app-bucket/tillandsia/my-app
      region: nyc3
    domain: my-app.example.com                # DNS record created automatically
```

You can edit this file directly, though the CLI commands are preferred.

## Agent-Friendly Usage

```bash
# JSON output:
tillandsia server list --json
tillandsia server inspect my-vps --json

# Non-interactive setup:
tillandsia server setup my-vps --yes
```