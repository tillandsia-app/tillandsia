# Cron Service Schemas

`Services` is currently `map[string]string` — fine for `web: node server.js`
but insufficient for `cron` services that need a schedule expression.

**Idea:** Make services a map of structs instead:

```yaml
services:
  web:
    command: node server.js
  worker:
    command: node worker.js
  daily-report:
    command: node report.js
    schedule: "0 6 * * *"
```

The `schedule` field is only valid/required for `cron` type services. The current
`map[string]string` shorthand (`web: node server.js`) could remain as syntactic
sugar for simple services with no extra metadata.

This changes the `Config.Services` type from `map[string]string` to
`map[string]ServiceConfig` where `ServiceConfig` has:

```go
type ServiceConfig struct {
    Command  string `yaml:"command" json:"command"`
    Schedule string `yaml:"schedule,omitempty" json:"schedule,omitempty"`
}
```

**Not yet implemented.** Worth doing before we build the init system's cron
support, since the data model feeds directly into process supervision.