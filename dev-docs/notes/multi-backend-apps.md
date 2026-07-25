# Multi-Backend Apps

Some apps package multiple backends (e.g. a web app + background worker + admin panel) in a single container. We need a convention for routing traffic to the right process.

**Proposal:** A `backends.yml` in the app root maps port numbers to backend names. Caddy inside the container proxies based on these mappings.

```yaml
# backends.yml
backends:
  - name: web
    port: 8080
  - name: worker
    port: 8081
  - name: admin
    port: 8082
```

The builder would generate a Caddyfile that reverse-proxies each declared port to the app's handler on that port, and the platform health-checks/exposes each backend independently.