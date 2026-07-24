# Python Quickstart

Deploy a Python application with Tillandsia.

## 1. Create Your Project

```bash
mkdir my-app && cd my-app
python -m venv venv
source venv/bin/activate
pip install flask gunicorn
pip freeze > requirements.txt
```

Create `app.py`:

```python
import os
from flask import Flask

app = Flask(__name__)
port = int(os.environ.get('PORT', 8080))

@app.route('/')
def hello():
    return 'Hello from Tillandsia!'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=port)
```

## 2. Initialize Tillandsia

```bash
tillandsia init
```

This detects `requirements.txt` and generates:
- `Dockerfile` — Python 3.12 slim image
- `Procfile` — `web: gunicorn -b 0.0.0.0:$PORT app:app`
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

Your app is running at `http://<your-vps-ip>:80`.