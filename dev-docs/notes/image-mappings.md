# Image Mappings

The builder registry in `internal/build/runtime.go` has Dockerfile templates
hardcoded as Go string constants. This works but isn't great for:

1. Users who want to add their own runtime
2. Community contributions adding new runtimes
3. Changing base image versions without a code release

**Idea:** Pull the runtime → image mapping into a JSON (or YAML) config file
embedded alongside the binary, or fetched at build time. Something like:

```json
{
  "node:20": {
    "image": "node:20-alpine",
    "build_steps": [
      "COPY package*.json ./",
      "RUN npm ci --only=production"
    ],
    "start": "node"
  }
}
```

This also makes it easy to add e.g. `node:18` or `deno` without touching Go code.

**Not urgent.** The current approach is fine for the MVP.