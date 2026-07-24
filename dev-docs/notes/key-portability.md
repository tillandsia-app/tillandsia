# Key Portability & Machine Migration

When switching to a new development machine, the SSH keys stored in
`~/.config/tillandsia/ssh/` don't move with you. The server config
(`servers.yaml`) — which holds host, user, provider info, and Litestream
credentials — is also local-only by default.

## Future Solution: Bucket-as-Roaming-Config

For Phase 6+ (or earlier if needed), sync `servers.yaml` to the app's
S3/Spaces bucket alongside the Litestream data. The bucket key could be:

```
tillandsia/<app-name>/config/servers.yaml
```

Migration flow on a new machine:

1. User authenticates with the same cloud provider (same API token)
2. `tillandsia server restore` reads configs from the bucket
3. A new keypair is generated on the new machine
4. The new public key is pushed to each server:
   - **Provisioned servers** — use the provider's API to replace the SSH key
     (e.g. DigitalOcean droplet `ssh_keys` update)
   - **Self-added servers** — prompt the user to install the new key, or
     attempt a one-time SSH dance using the old private key (if it was
     migrated manually)

## Alternative: Config File Portability

Simpler path for self-managed users: just copy `~/.config/tillandsia/` to
the new machine and run `tillandsia server setup` to re-push the existing
public key. No bucket needed, but manual.

## Status

Not yet implemented. Documented for Phase 6 planning.