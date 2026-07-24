package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tillandsia/tillandsia/internal/ssh"
)

func Setup(ctx context.Context, host, user, keyPath string, port int) error {
	client, err := ssh.Connect(host, user, keyPath, port)
	if err != nil {
		return fmt.Errorf("connecting to server: %w", err)
	}
	defer client.Close()

	if err := installDocker(ctx, client); err != nil {
		return err
	}

	if err := createDataDir(ctx, client); err != nil {
		return err
	}

	if err := verifyDocker(ctx, client); err != nil {
		return err
	}

	return nil
}

func installDocker(ctx context.Context, client *ssh.Client) error {
	// Check if Docker is already installed
	result, err := client.Run(ctx, "docker info --format '{{.ServerVersion}}'")
	if err != nil || result.ExitCode != 0 {
		// Docker not installed — install it
		result, err = client.Run(ctx, detectInstallCmd())
		if err != nil {
			return fmt.Errorf("running Docker install: %w", err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("Docker install failed:\n%s%s", result.Stdout, result.Stderr)
		}
	}

	return nil
}

func detectInstallCmd() string {
	return `curl -fsSL https://get.docker.com | sh`
}

func createDataDir(ctx context.Context, client *ssh.Client) error {
	result, err := client.Run(ctx, "mkdir -p /var/lib/tillandsia && echo ok")
	if err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("creating data directory failed:\n%s", result.Stderr)
	}
	return nil
}

func verifyDocker(ctx context.Context, client *ssh.Client) error {
	result, err := client.Run(ctx, "docker run --rm hello-world 2>&1 | head -3")
	if err != nil {
		errStr := ""
		if result != nil {
			errStr = result.Stderr
		}
		if strings.Contains(err.Error()+errStr, "cannot connect to the Docker daemon") {
			return fmt.Errorf("Docker is installed but the daemon is not running. Try rebooting the server.")
		}
		return fmt.Errorf("Docker verification failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("Docker daemon is not running:\n%s", result.Stderr)
	}
	return nil
}

func Test(ctx context.Context, host, user, keyPath string, port int) error {
	client, err := ssh.Connect(host, user, keyPath, port)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Test(ctx)
}

func CreateAppDir(ctx context.Context, client *ssh.Client, appName string) error {
	result, err := client.Run(ctx, fmt.Sprintf("mkdir -p /var/lib/tillandsia/%s/data && echo ok", appName))
	if err != nil {
		return fmt.Errorf("creating app directory: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("creating app directory failed:\n%s", result.Stderr)
	}
	return nil
}

const HealthCheckTimeout = 30 * time.Second

func WaitForReady(ctx context.Context, host string, port int) error {
	ctx, cancel := context.WithTimeout(ctx, HealthCheckTimeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timed out after %s", HealthCheckTimeout)
		case <-ticker.C:
			// Use SSH to check container health via management API
			// For now, just check the container is running via Docker
			client, err := ssh.Connect(host, "root", "", 22)
			if err != nil {
				continue
			}
			result, err := client.Run(ctx, fmt.Sprintf("docker inspect --format '{{.State.Health.Status}}' tillandsia-%s 2>/dev/null || docker inspect --format '{{.State.Status}}' tillandsia-%s 2>/dev/null || echo not-found", host, host))
			client.Close()
			if err != nil {
				continue
			}
			status := strings.TrimSpace(result.Stdout)
			if status == "healthy" || status == "running" {
				return nil
			}
			os.Stderr.WriteString(fmt.Sprintf("  waiting for container (status: %s)...\n", status))
		}
	}
}