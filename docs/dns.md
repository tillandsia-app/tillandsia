# DNS & TLS

Tillandsia uses Caddy for automatic TLS. All TLS is handled by Caddy inside the
container — no external service required.

## How It Works

Caddy supports the **HTTP-01 ACME challenge** out of the box. When your app starts:

1. Caddy sees the domain configured in `tillandsia.yaml`.
2. It verifies ownership by serving a challenge file on port 80.
3. Let's Encrypt issues a certificate.
4. Caddy terminates TLS and proxies traffic to your web service.

This is fully automatic. No certbot, no manual renewal, no configuration.

## Setting Up a Domain (Manual Server)

If you added your server with `tillandsia server add`, you manage DNS yourself:

1. **Buy a domain** from any registrar (Namecheap, Cloudflare, Porkbun, etc.).
2. **Create an A record** pointing to your VPS IP address:
   ```
   Type:  A
   Name:  my-app
   Value: <your-vps-ip>
   TTL:   300
   ```
3. **Configure your domain** in `tillandsia.yaml`:
   ```yaml
   domain: my-app.example.com
   ```
4. **Deploy:**
   ```bash
   tillandsia deploy
   ```

Caddy handles the rest. Your app will be available at `https://my-app.example.com`
within seconds.

## Setting Up a Domain (Provisioned Server)

If you used `tillandsia server provision do`, DNS is handled automatically:

1. Pass the domain when provisioning:
   ```bash
   tillandsia server provision do --name my-app --domain my-app.example.com
   ```
2. The provider creates the A record pointing to the VPS IP and stores the domain
   in the server config.
3. You don't need to touch your DNS provider's control panel at all.

## No Domain? No Problem

If you don't configure a domain, the app is served on the server's IP address
over HTTP (no TLS). This is fine for testing, internal tools, or APIs consumed
by other services.

## How Caddy Routes Traffic

The init system generates a Caddy config that looks like:

```
my-app.example.com {
    reverse_proxy localhost:8080
}
```

If no domain is configured, it generates:

```
:80 {
    reverse_proxy localhost:8080
}
```

The `$PORT` (or configured port) is used as the upstream target.

## Caddyfile Customization

For advanced use cases, you can provide a custom `Caddyfile` in your project root.
If present, the init system uses it instead of the generated config.

## Port Requirements

- **Port 80** — Required for HTTP-01 ACME challenges (Let's Encrypt verification).
- **Port 443** — Required for HTTPS traffic.
- **Port 22** — Required for SSH (Tillandsia CLI).

Make sure your VPS firewall allows inbound traffic on these ports.

## Future: Hosted ACME Zone

In a future version, Tillandsia will offer a hosted `*.tillandsia.app` DNS zone.
With DNS-01 ACME challenges, you'd get a `your-app.tillandsia.app` subdomain
automatically — no domain purchase, no DNS record creation. The certificate
would be issued and renewed centrally, with no action from you.

This is currently planned as a paid feature.