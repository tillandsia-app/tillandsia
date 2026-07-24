# Go Quickstart

Deploy a Go application with Tillandsia.

## 1. Create Your Project

```bash
mkdir my-app && cd my-app
go mod init my-app
```

Create `main.go`:

```go
package main

import (
    "fmt"
    "net/http"
    "os"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from Tillandsia!")
    })

    fmt.Printf("Server listening on port %s\n", port)
    http.ListenAndServe(":"+port, nil)
}
```

## 2. Initialize Tillandsia

```bash
tillandsia init
```

This detects `go.mod` and generates:
- `Dockerfile` — Multi-stage build (Go builder → Alpine runtime)
- `Procfile` — `web: ./app`
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