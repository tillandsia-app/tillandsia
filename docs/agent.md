# Agent-Friendly CLI

Tillandsia is designed to be used by both humans and AI agents. Every command
supports structured output, non-interactive mode, and consistent exit codes.

## Global Flags

```
--json       Output results as JSON (machine-readable)
--yes, -y    Skip all confirmation prompts
--quiet, -q  Suppress human-friendly output (spinners, progress)
```

## Output Convention

- **stdout** — Structured data only (JSON, export output, etc.)
- **stderr** — Human-friendly output (spinners, progress bars, logs)

This means agents can always parse stdout without noise:

```bash
# Agent: get server list as JSON
tillandsia server list --json | jq '.[0].host'

# Agent: check deploy status
tillandsia status --json | jq '.healthy'

# Agent: export env vars
tillandsia env export --json | jq '.DATABASE_URL'
```

## JSON Output Format

Every command that returns data supports `--json`. The format is consistent:

```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

On error:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "SERVER_UNREACHABLE",
    "message": "Could not connect to server my-vps at 203.0.113.42:22"
  }
}
```

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | General error |
| 2 | Configuration error (invalid YAML, missing file) |
| 3 | Server unreachable or SSH connection failed |
| 4 | Build failure (Docker build failed) |
| 5 | Deploy failure (container failed to start) |
| 6 | Authentication error (SSH key, env var access) |

## Commands Reference

### `tillandsia deploy`

```bash
# Agent: deploy with explicit config
tillandsia deploy --server my-vps --yes --json

# Agent: dry run
tillandsia deploy --dry-run --json
```

Output:

```json
{
  "success": true,
  "data": {
    "app": "my-app",
    "server": "my-vps",
    "image": "sha256:abc123...",
    "url": "https://my-app.example.com",
    "healthy": true,
    "startup_ms": 3420
  }
}
```

### `tillandsia status`

```bash
# Agent: get full status
tillandsia status --json
```

Output:

```json
{
  "success": true,
  "data": {
    "app": "my-app",
    "server": "my-vps",
    "deployed": true,
    "healthy": true,
    "uptime_seconds": 86400,
    "image": "sha256:abc123...",
    "litestream_lag_seconds": 0.5,
    "env_count": 12
  }
}
```

### `tillandsia env`

```bash
# Agent: set a variable
tillandsia env set DATABASE_URL=postgres://... --json

# Agent: get a variable
tillandsia env get DATABASE_URL --json

# Agent: export all variables
tillandsia env export --json
```

Output for `env export --json`:

```json
{
  "success": true,
  "data": {
    "DATABASE_URL": "postgres://...",
    "API_KEY": "sk-abc123",
    "NODE_ENV": "production"
  }
}
```

### `tillandsia server`

```bash
# Agent: add a server
tillandsia server add my-vps --host 203.0.113.42 --json

# Agent: list servers
tillandsia server list --json

# Agent: inspect a server
tillandsia server inspect my-vps --json
```

Output for `server list --json`:

```json
{
  "success": true,
  "data": [
    {
      "name": "my-vps",
      "host": "203.0.113.42",
      "user": "root",
      "port": 22,
      "default": true,
      "setup": true
    }
  ]
}
```

### `tillandsia logs`

```bash
# Agent: get recent logs as JSON lines
tillandsia logs --tail 10 --json
```

Output (one JSON object per line on stdout):

```json
{"timestamp": "2025-01-01T00:00:00Z", "stream": "stdout", "message": "Server started on port 8080"}
{"timestamp": "2025-01-01T00:00:01Z", "stream": "stdout", "message": "GET / 200 12ms"}
```

### `tillandsia init`

```bash
# Agent: scaffold a project without interaction
tillandsia init --language node --yes --json
```

Output:

```json
{
  "success": true,
  "data": {
    "files": ["Dockerfile", "Procfile", "tillandsia.yaml"],
    "language": "node",
    "directory": "/path/to/project"
  }
}
```

## Dry-Run Mode

`tillandsia deploy --dry-run` shows what would change without actually deploying:

```json
{
  "success": true,
  "data": {
    "app": "my-app",
    "server": "my-vps",
    "actions": [
      {"action": "build", "detail": "docker build -t tillandsia/my-app ."},
      {"action": "transfer", "detail": "docker save tillandsia/my-app | ssh ... docker load"},
      {"action": "stop", "detail": "docker stop my-app (existing container)"},
      {"action": "start", "detail": "docker run --name my-app ... tillandsia/my-app"}
    ]
  }
}
```

## Environment Variable Overrides

All CLI flags can be set via environment variables prefixed with `TILLANDSIA_`:

```bash
export TILLANDSIA_SERVER=my-vps
export TILLANDSIA_FORMAT=json
export TILLANDSIA_YES=true

tillandsia deploy  # equivalent to: tillandsia deploy --server my-vps --json --yes
```

## State File

Tillandsia stores its state in `~/.config/tillandsia/`:

```
~/.config/tillandsia/
├── servers.yaml     # Server configurations
├── state.yaml       # Deploy state per app
└── env/             # Cached env files (optional)
```

Agents can read these files directly for a full picture of the current state.