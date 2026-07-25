package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

const MgmtPort = 8080

type ManagementServer struct {
	supervisor *Supervisor
	port       int
	server     *http.Server
	startTime  time.Time
}

func NewManagementServer(supervisor *Supervisor, appPort int) *ManagementServer {
	return &ManagementServer{
		supervisor: supervisor,
		startTime:  time.Now(),
	}
}

func (ms *ManagementServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mgmt/health", ms.handleHealth)
	mux.HandleFunc("/mgmt/shutdown", ms.handleShutdown)
	mux.HandleFunc("/mgmt/identity", ms.handleIdentity)

	// Also serve root for basic health
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ms.server = &http.Server{
		Handler: mux,
		Addr:    fmt.Sprintf(":%d", MgmtPort),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "management API: %v\n", err)
		}
	}()

	return nil
}

func (ms *ManagementServer) Stop() {
	if ms.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ms.server.Shutdown(shutdownCtx)
	}
}

func (ms *ManagementServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := ms.supervisor.Health()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (ms *ManagementServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})

	go func() {
		time.Sleep(100 * time.Millisecond)
		ms.supervisor.Shutdown()
	}()
}

func (ms *ManagementServer) handleIdentity(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	info := map[string]interface{}{
		"hostname":   hostname,
		"started_at": ms.startTime.UTC().Format(time.RFC3339),
		"uptime":     time.Since(ms.startTime).String(),
		"port":       mgmtPort(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func mgmtPort() int {
	return MgmtPort
}

func (ms *ManagementServer) Port() int {
	return ms.port
}
