package supervisor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const CaddyfilePath = "/etc/caddy/Caddyfile"

type CaddyManager struct {
	domain    string
	port      int
	customCaddy string
	cmd       *exec.Cmd
	running   bool
	logger    *log.Logger
}

func NewCaddyManager(domain string, port int, customCaddy string, logger *log.Logger) *CaddyManager {
	if port == 0 {
		port = 8080
	}
	return &CaddyManager{
		domain:      domain,
		port:        port,
		customCaddy: customCaddy,
		logger:      logger,
	}
}

func (cm *CaddyManager) GenerateCaddyfile() string {
	if cm.customCaddy != "" {
		return cm.customCaddy
	}

	addr := fmt.Sprintf("localhost:%d", cm.port)

	if cm.domain != "" {
		return fmt.Sprintf(`%s {
	reverse_proxy %s
}
`, cm.domain, addr)
	}

	return fmt.Sprintf(`:80 {
	reverse_proxy %s
}
`, addr)
}

func (cm *CaddyManager) WriteCaddyfile() error {
	content := cm.GenerateCaddyfile()
	dir := filepath.Dir(CaddyfilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating caddy directory: %w", err)
	}
	return os.WriteFile(CaddyfilePath, []byte(content), 0644)
}

func (cm *CaddyManager) Start(ctx context.Context) error {
	if err := cm.WriteCaddyfile(); err != nil {
		return fmt.Errorf("writing caddyfile: %w", err)
	}

	cm.cmd = exec.CommandContext(ctx, "caddy", "run", "--config", CaddyfilePath)
	cm.cmd.Stdout = os.Stdout
	cm.cmd.Stderr = os.Stderr
	cm.cmd.Env = os.Environ()

	if err := cm.cmd.Start(); err != nil {
		return fmt.Errorf("starting caddy: %w", err)
	}

	cm.running = true

	// Monitor in background
	go func() {
		if err := cm.cmd.Wait(); err != nil {
			cm.logger.Printf("caddy exited: %v", err)
		}
		cm.running = false
	}()

	// Give Caddy a moment to start
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (cm *CaddyManager) Stop(ctx context.Context) error {
	if cm.cmd != nil && cm.cmd.Process != nil {
		if err := cm.cmd.Process.Signal(os.Interrupt); err != nil {
			cm.cmd.Process.Kill()
		}
		done := make(chan struct{})
		go func() {
			cm.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			cm.cmd.Process.Kill()
		}
	}
	cm.running = false
	return nil
}

func (cm *CaddyManager) IsRunning() bool {
	return cm.running
}

// ZeroDowntimeHandoff checks if an old container is running on the management
// port and asks it to shut down gracefully.
func ZeroDowntimeHandoff(ctx context.Context, mgmtPort int, appName string) error {
	// Check for existing management server on the given port
	// This is a simple TCP check + HTTP request
	// In practice, the old container's management API would be at
	// the old container's IP:8080/mgmt/shutdown

	// For now, this is a stub that will be fleshed out in Phase 6
	return nil
}

func detectDomainFromEnv() string {
	domain := os.Getenv("TILLANDSIA_DOMAIN")
	if domain != "" {
		return domain
	}
	domain = os.Getenv("CADDY_DOMAIN")
	if domain != "" {
		return domain
	}
	return ""
}

func detectCustomCaddy() string {
	if _, err := os.Stat("/etc/caddy/Caddyfile.custom"); err == nil {
		data, err := os.ReadFile("/etc/caddy/Caddyfile.custom")
		if err == nil {
			return string(data)
		}
	}

	custom := os.Getenv("TILLANDSIA_CADDYFILE")
	if custom != "" {
		return custom
	}

	return ""
}

func readCaddyfile() (string, error) {
	if _, err := os.Stat(CaddyfilePath); err == nil {
		data, err := os.ReadFile(CaddyfilePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", fmt.Errorf("caddyfile not found at %s", CaddyfilePath)
}

func cleanEnvName(s string) string {
	return strings.NewReplacer(
		"-", "_",
		".", "_",
		" ", "_",
	).Replace(strings.ToUpper(s))
}
