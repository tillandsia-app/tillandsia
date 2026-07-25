package supervisor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	DefaultHealthPath    = "/"
	HealthCheckInterval  = 2 * time.Second
	HealthCheckTimeout   = 60 * time.Second
)

type HealthChecker struct {
	port       int
	healthPath string
	client     *http.Client
}

func NewHealthChecker(port int, healthPath string) *HealthChecker {
	if healthPath == "" {
		healthPath = DefaultHealthPath
	}
	return &HealthChecker{
		port:       port,
		healthPath: healthPath,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
			},
		},
	}
}

func (hc *HealthChecker) Wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, HealthCheckTimeout)
	defer cancel()

	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timed out after %s", HealthCheckTimeout)
		case <-ticker.C:
			url := fmt.Sprintf("http://localhost:%d%s", hc.port, hc.healthPath)
			resp, err := hc.client.Get(url)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
	}
}

func (hc *HealthChecker) Check() error {
	url := fmt.Sprintf("http://localhost:%d%s", hc.port, hc.healthPath)
	resp, err := hc.client.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}
	return fmt.Errorf("health check returned status %d", resp.StatusCode)
}

type HealthStatus struct {
	Liveness      string            `json:"liveness"`
	Readiness     string            `json:"readiness"`
	Services      map[string]string `json:"services"`
	CaddyRunning  bool              `json:"caddy_running"`
	LitestreamLag int64             `json:"litestream_lag"`
}
