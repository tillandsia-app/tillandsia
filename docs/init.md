# Project Scaffolding

`tillandsia init` scaffolds a new project with everything you need to deploy.
It detects your project's language and generates the appropriate files.

## Usage

```bash
# In an existing project:
tillandsia init

# In an empty directory (creates a sample app):
tillandsia init

# Force a specific language:
tillandsia init --language go
tillandsia init --language python
tillandsia init --language node
tillandsia init --language static
tillandsia init --language ruby
```

## Language Detection

`tillandsia init` detects your project's language by looking for these files:

| Language | Detection File | Generated Files |
|---|---|---|
| Node.js | `package.json` | `Dockerfile`, `Procfile`, `tillandsia.yaml` |
| Python | `requirements.txt`, `pyproject.toml` | `Dockerfile`, `Procfile`, `tillandsia.yaml` |
| Go | `go.mod` | `Dockerfile`, `Procfile`, `tillandsia.yaml` |
| Ruby | `Gemfile` | `Dockerfile`, `Procfile`, `tillandsia.yaml` |
| Static HTML | `index.html` | `Dockerfile` (nginx), `Procfile`, `tillandsia.yaml` |
| Unknown | — | `Dockerfile`, `Procfile`, `tillandsia.yaml` (generic) |

## What Gets Generated

### `tillandsia.yaml`

```yaml
name: my-app
port: 8080
env:
  NODE_ENV: production
build:
  dockerfile: Dockerfile
  context: .
```

### `Procfile`

```yaml
web: npm start
```

### `Dockerfile` (language-specific)

For Node.js:

```dockerfile
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 8080
CMD ["node", "server.js"]
```

For Python:

```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8080
CMD ["python", "app.py"]
```

For Go:

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o app .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/app .
EXPOSE 8080
CMD ["./app"]
```

### Sample App (empty directory only)

If the directory is empty, `tillandsia init` also generates a minimal hello-world
application so you can deploy immediately:

- `server.js` — A simple HTTP server
- `index.html` — A basic HTML page

## What `tillandsia init` Does Not Do

- It does not create a server (`tillandsia server add`).
- It does not set up environment variables (`tillandsia env set`).
- It does not deploy (`tillandsia deploy`).

It only scaffolds the files you need in your project directory.

## After `tillandsia init`

```bash
# 1. Add a server
tillandsia server add my-vps --host <ip>

# 2. Set up the server
tillandsia server setup my-vps

# 3. Configure environment variables
tillandsia env set DATABASE_URL=postgres://...

# 4. Deploy
tillandsia deploy

# 5. Check logs
tillandsia logs
```