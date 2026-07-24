package supervisor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	LitestreamConfigPath = "/etc/litestream.yml"
	LitestreamRestoreTimeout = 60 * time.Second
	DataDir             = "/data"
	DBPath              = "/data/db"
)

type LitestreamEnv struct {
	AccessKeyID     string
	SecretAccessKey string
	URL             string
	Region          string
}

type LitestreamManager struct {
	env     *LitestreamEnv
	cmd     *exec.Cmd
	running bool
	logger  *log.Logger
	lag     int64
}

func NewLitestreamManager(env *LitestreamEnv, logger *log.Logger) *LitestreamManager {
	return &LitestreamManager{
		env:    env,
		logger: logger,
	}
}

func (lm *LitestreamManager) GenerateConfig() string {
	path := filepath.Join(DataDir, "db")
	config := fmt.Sprintf(`dbs:
  - path: %s
    replicas:
      - url: %s
`, path, lm.env.URL)

	return config
}

func (lm *LitestreamManager) WriteConfig() error {
	content := lm.GenerateConfig()
	dir := filepath.Dir(LitestreamConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating litestream config directory: %w", err)
	}
	return os.WriteFile(LitestreamConfigPath, []byte(content), 0644)
}

func (lm *LitestreamManager) Restore(ctx context.Context) error {
	if lm.env.URL == "" {
		lm.logger.Println("litestream: no replica URL configured, skipping restore")
		return nil
	}

	if err := lm.WriteConfig(); err != nil {
		return fmt.Errorf("writing litestream config: %w", err)
	}

	// Check if DB already exists
	if _, err := os.Stat(DBPath); err == nil {
		lm.logger.Println("litestream: database already exists, skipping restore")
		return nil
	}

	restoreCtx, cancel := context.WithTimeout(ctx, LitestreamRestoreTimeout)
	defer cancel()

	lm.logger.Println("litestream: restoring from latest snapshot...")
	cmd := exec.CommandContext(restoreCtx, "litestream", "restore",
		"-config", LitestreamConfigPath,
		"-if-db-not-exists",
		"-if-replica-exists",
		"-o", DBPath,
		lm.env.URL,
	)
	cmd.Env = lm.buildEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Not a hard failure - the app can still start without a DB restore
		lm.logger.Printf("litestream: restore warning (starting fresh): %v", err)
		return nil
	}

	lm.logger.Println("litestream: restore complete")
	return nil
}

func (lm *LitestreamManager) StartReplication(ctx context.Context) error {
	if lm.env.URL == "" {
		return nil
	}

	cmd := exec.CommandContext(ctx, "litestream", "replicate",
		"-config", LitestreamConfigPath,
	)
	cmd.Env = lm.buildEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting litestream replication: %w", err)
	}

	lm.cmd = cmd
	lm.running = true

	go func() {
		if err := cmd.Wait(); err != nil {
			lm.logger.Printf("litestream: replication exited: %v", err)
		}
		lm.running = false
	}()

	time.Sleep(500 * time.Millisecond)
	return nil
}

func (lm *LitestreamManager) Flush(ctx context.Context) error {
	if lm.env.URL == "" {
		return nil
	}

	lm.logger.Println("litestream: flushing WAL...")
	cmd := exec.CommandContext(ctx, "litestream", "snapshot",
		"-config", LitestreamConfigPath,
		DBPath,
	)
	cmd.Env = lm.buildEnv()
	if err := cmd.Run(); err != nil {
		lm.logger.Printf("litestream: flush warning: %v", err)
	}
	return nil
}

func (lm *LitestreamManager) Stop() {
	if lm.cmd != nil && lm.cmd.Process != nil {
		lm.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			lm.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			lm.cmd.Process.Kill()
		}
	}
	lm.running = false
}

func (lm *LitestreamManager) IsRunning() bool {
	return lm.running
}

func (lm *LitestreamManager) Lag() int64 {
	return lm.lag
}

func (lm *LitestreamManager) buildEnv() []string {
	env := os.Environ()
	if lm.env.AccessKeyID != "" {
		env = append(env, fmt.Sprintf("LITESTREAM_ACCESS_KEY_ID=%s", lm.env.AccessKeyID))
	}
	if lm.env.SecretAccessKey != "" {
		env = append(env, fmt.Sprintf("LITESTREAM_SECRET_ACCESS_KEY=%s", lm.env.SecretAccessKey))
	}
	if lm.env.Region != "" {
		env = append(env, fmt.Sprintf("LITESTREAM_REGION=%s", lm.env.Region))
	}
	return env
}
