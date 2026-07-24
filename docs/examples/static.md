# Static Site Quickstart

Deploy a static site with Tillandsia.

## 1. Create Your Project

```bash
mkdir my-app && cd my-app
```

Create `index.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>My Tillandsia Site</title>
</head>
<body>
    <h1>Hello from Tillandsia!</h1>
    <p>This is a static site.</p>
</body>
</html>
```

## 2. Initialize Tillandsia

```bash
tillandsia init
```

This detects `index.html` and generates:
- `Dockerfile` — Nginx Alpine serving static files
- `Procfile` — `web: nginx -g 'daemon off;'`
- `tillandsia.yaml` — Project configuration

## 3. Add a Server

```bash
tillandsia server add my-vps --host <your-vps-ip>
tillandsia server setup my-vps
```

## 4. Deploy

```bash
tillandsia deploy
```

## 5. Verify

```bash
tillandsia logs
tillandsia status
```

Your static site is running at `http://<your-vps-ip>:80`.