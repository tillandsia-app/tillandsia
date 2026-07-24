# Environment Variables & Secrets

Environment variables are stored on the server in an env file at
`/var/lib/tillandsia/<app-name>/env`. They are loaded into the container at
startup and made available to all processes.

## Commands

```bash
# Set or update a variable:
tillandsia env set DATABASE_URL=postgres://...
tillandsia env set API_KEY=sk-abc123

# Get a variable's value:
tillandsia env get DATABASE_URL

# Remove a variable:
tillandsia env rm API_KEY

# List all variables (names only by default):
tillandsia env list

# Export all variables in key=value format:
tillandsia env export

# Export as JSON (for agents):
tillandsia env export --json
```

## How It Works

1. Variables are stored in a plaintext env file on the server.
2. The file is owned by root and only readable by the tillandsia user.
3. When the container starts, the env file is mounted into the container at
   `/run/tillandsia/env` and sourced by the init system.
4. **After changing env vars, the container is automatically restarted** so the
   new values take effect.

## Static vs Secret Variables

### Static variables (in `tillandsia.yaml`)

```yaml
env:
  NODE_ENV: production
  LOG_LEVEL: info
```

These are committed to your repository. Use them for non-sensitive configuration.

### Secret variables (via `tillandsia env set`)

```bash
tillandsia env set DATABASE_URL=postgres://user:pass@host/db
```

These are stored only on the server, never in your repo. Use them for secrets.

### Variable precedence

Secret variables (on the server) override static variables (in `tillandsia.yaml`).
This lets you commit safe defaults and override secrets per-environment.

## Multi-Environment Management

When you have multiple servers (staging, production), you can manage env vars
per-server:

```bash
tillandsia env set DATABASE_URL=... --server staging
tillandsia env set DATABASE_URL=... --server production
```

## Env File Format

The env file on the server is a simple `KEY=VALUE` format:

```
DATABASE_URL=postgres://user:pass@host/db
API_KEY=sk-abc123
```

## Security Notes

- The env file is stored on disk on the VPS. If someone gains root access to your
  VPS, they can read your secrets.
- For production workloads, consider using a secrets manager like HashiCorp Vault
  or a cloud KMS. Tillandsia's env system is designed for convenience, not for
  high-security environments.
- The env file is never transferred over the network except during SSH sessions
  (which are encrypted).