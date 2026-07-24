# Node.js Quickstart

Deploy a Node.js application with Tillandsia.

## 1. Create Your Project

```bash
mkdir my-app && cd my-app
npm init -y
npm install express
```

Create `server.js`:

```javascript
const express = require('express');
const app = express();
const port = process.env.PORT || 8080;

app.get('/', (req, res) => {
  res.send('Hello from Tillandsia!');
});

app.listen(port, () => {
  console.log(`Server listening on port ${port}`);
});
```

## 2. Initialize Tillandsia

```bash
tillandsia init
```

This detects `package.json` and generates:
- `Dockerfile` — Multi-stage build with Node 20 Alpine
- `Procfile` — `web: node server.js`
- `tillandsia.yaml` — Project configuration

## 3. Add a Server

```bash
tillandsia server add my-vps --host <your-vps-ip>
tillandsia server setup my-vps
```

## 4. Configure Environment Variables

```bash
tillandsia env set NODE_ENV=production
```

## 5. Deploy

```bash
tillandsia deploy
```

## 6. Verify

```bash
tillandsia logs
tillandsia status
```

Your app is now running at `http://<your-vps-ip>:80`. Configure a domain and
deploy again for HTTPS.