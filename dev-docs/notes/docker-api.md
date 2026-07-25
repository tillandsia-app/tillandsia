# Docker API vs CLI

Currently the deploy pipeline shells out to `docker` CLI commands (both
locally and over SSH). The alternative is using the official Go Docker SDK
(`github.com/docker/docker/client`).

## Benefits

- No shell injection surface (arg structs instead of string formatting)
- Structured errors instead of parsing CLI stderr
- Streaming APIs for logs, events, and image transfers
- Native SSH transport — SDK can connect to remote daemons via
  `ssh://user@host` URLs, so local and remote code paths converge

## Effort breakdown

| Area | Effort | Notes |
|------|--------|-------|
| `docker build` | 15 min | `ImageBuild()` — straightforward swap |
| `docker save` / `docker load` | 15 min | `ImageSave()` returns a reader, `ImageLoad()` accepts one |
| `docker inspect` | 15 min | `ContainerInspect()` — struct return |
| `docker stop` / `docker rm` / `docker restart` | 15 min | Trivial one-liners in SDK |
| `docker run` | 1 hr | `ContainerCreate()` with mounts, env, ports, restart policy |
| `docker logs --follow` | 1 hr | `ContainerLogs()` with streaming — need to handle frame protocol |
| SSH transport wiring | 1 hr | SDK's `ssh` dialer, connection pooling, key auth |
| Cleanup & testing | 30 min | |

**Total SWAG: ~4 hours.**

## Tradeoffs

- Adds `docker/docker` and all its dependencies to `go.mod` (it pulls in a lot)
- The SDK's SSH transport uses `golang.org/x/crypto/ssh` which we already have
- `docker run` maps to a large `ContainerCreate` struct — less readable than the
  shell string but type-safe

Not urgent — the current CLI approach works and shell injection is mitigated by
the app name validation.