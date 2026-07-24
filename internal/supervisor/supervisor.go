package supervisor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	DefaultRetryLimit = 3
	ShutdownTimeout   = 15 * time.Second
)

type ServiceType string

const (
	ServiceTypeWeb    ServiceType = "web"
	ServiceTypeWorker ServiceType = "worker"
	ServiceTypeCron   ServiceType = "cron"
)

type InitConfig struct {
	Name         string
	Port         int
	Domain       string
	Services     map[string]string
	Env          map[string]string
	Litestream   *LitestreamEnv
	CustomCaddy  string
	RetryLimit   int
	HealthPath   string
}

type managedProcess struct {
	name    string
	typ     ServiceType
	cmd     string
	cmdline *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
	retries int
	running bool
}

type Supervisor struct {
	cfg     *InitConfig
	procs   []*managedProcess
	litestream *LitestreamManager
	caddy   *CaddyManager
	health  *HealthChecker
	mgmt    *ManagementServer
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	logger  *log.Logger
}

func NewSupervisor(cfg *InitConfig) *Supervisor {
	if cfg.RetryLimit == 0 {
		cfg.RetryLimit = DefaultRetryLimit
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}
	return &Supervisor{
		cfg:    cfg,
		done:   make(chan struct{}),
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (s *Supervisor) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Start Litestream if configured
	if s.cfg.Litestream != nil {
		s.litestream = NewLitestreamManager(s.cfg.Litestream, s.logger)
		if err := s.litestream.Restore(s.ctx); err != nil {
			s.logger.Printf("litestream restore: %v", err)
		}
		if err := s.litestream.StartReplication(s.ctx); err != nil {
			return fmt.Errorf("starting litestream replication: %w", err)
		}
	}

	// Start user services
	for name, cmd := range s.cfg.Services {
		typ := ServiceType(strings.SplitN(name, ":", 2)[0])
		mp := &managedProcess{
			name: name,
			typ:  ServiceType(typ),
			cmd:  cmd,
		}
		s.procs = append(s.procs, mp)
		go s.runProcess(mp)
	}

	// Create health checker
	s.health = NewHealthChecker(s.cfg.Port, s.cfg.HealthPath)

	// Start management API
	s.mgmt = NewManagementServer(s, s.cfg.Port)
	go func() {
		if err := s.mgmt.Start(s.ctx); err != nil {
			s.logger.Printf("management API: %v", err)
		}
	}()

	// Wait for web service health before starting Caddy
	webRunning := false
	for _, mp := range s.procs {
		if mp.typ == ServiceTypeWeb {
			webRunning = true
			break
		}
	}

	if webRunning {
		s.logger.Printf("waiting for web service to become healthy on port %d...", s.cfg.Port)
		if err := s.health.Wait(s.ctx); err != nil {
			s.logger.Printf("health check warning: %v", err)
		}
	}

	// Start Caddy
	s.caddy = NewCaddyManager(s.cfg.Domain, s.cfg.Port, s.cfg.CustomCaddy, s.logger)
	if err := s.caddy.Start(s.ctx); err != nil {
		return fmt.Errorf("starting caddy: %w", err)
	}

	s.logger.Println("init system ready")

	// Wait for shutdown signal
	select {
	case sig := <-sigCh:
		s.logger.Printf("received signal %v, shutting down", sig)
		s.Shutdown()
	case <-s.ctx.Done():
		s.Shutdown()
	case <-s.done:
	}

	return nil
}

func (s *Supervisor) runProcess(mp *managedProcess) {
	mp.mu.Lock()
	mp.running = true
	mp.mu.Unlock()

	for {
		procCtx, cancel := context.WithCancel(s.ctx)
		mp.cancel = cancel

		cmd := buildCommand(mp.cmd)
		cmd.Env = os.Environ()
		for k, v := range s.cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", s.cfg.Port))

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			s.logger.Printf("[%s] stdout pipe error: %v", mp.name, err)
			break
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			s.logger.Printf("[%s] stderr pipe error: %v", mp.name, err)
			break
		}

		if err := cmd.Start(); err != nil {
			s.logger.Printf("[%s] start error: %v", mp.name, err)
			break
		}

		mp.cmdline = cmd

		// Stream stdout/stderr
		var wg sync.WaitGroup
		wg.Add(2)
		go s.streamOutput(mp.name, stdout, &wg)
		go s.streamOutput(mp.name, stderr, &wg)

		waitErr := cmd.Wait()
		wg.Wait()

		mp.mu.Lock()
		mp.running = false
		mp.retries++
		retries := mp.retries
		mp.mu.Unlock()

		// Check if shutdown is requested
		select {
		case <-procCtx.Done():
			return
		default:
		}

		if waitErr != nil {
			s.logger.Printf("[%s] exited with error: %v", mp.name, waitErr)
		} else {
			s.logger.Printf("[%s] exited cleanly", mp.name)
		}

		if retries >= s.cfg.RetryLimit {
			s.logger.Printf("[%s] reached retry limit (%d), giving up", mp.name, s.cfg.RetryLimit)
			return
		}

		s.logger.Printf("[%s] restarting in 1s (attempt %d/%d)", mp.name, retries+1, s.cfg.RetryLimit)
		time.Sleep(time.Second)
	}
}

func (s *Supervisor) streamOutput(name string, r io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			line := string(buf[:n])
			for _, l := range strings.Split(line, "\n") {
				if l != "" {
					s.logger.Printf("[%s] %s", name, l)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *Supervisor) Shutdown() {
	s.logger.Println("init system shutting down...")

	// Stop Caddy first (stop accepting connections)
	if s.caddy != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		s.caddy.Stop(shutdownCtx)
		cancel()
	}

	// Stop user services
	var wg sync.WaitGroup
	for _, mp := range s.procs {
		wg.Add(1)
		go func(mp *managedProcess) {
			defer wg.Done()
			mp.mu.Lock()
			if mp.cmdline != nil && mp.cmdline.Process != nil {
				mp.cmdline.Process.Signal(syscall.SIGTERM)
				done := make(chan struct{})
				go func() {
					mp.cmdline.Wait()
					close(done)
				}()
				select {
				case <-done:
				case <-time.After(ShutdownTimeout):
					mp.cmdline.Process.Kill()
				}
			}
			mp.mu.Unlock()
		}(mp)
	}
	wg.Wait()

	// Flush Litestream
	if s.litestream != nil {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		s.litestream.Flush(flushCtx)
		cancel()
	}

	// Stop management API
	if s.mgmt != nil {
		s.mgmt.Stop()
	}

	s.cancel()
	close(s.done)
	s.logger.Println("init system shutdown complete")
}

func (s *Supervisor) Health() *HealthStatus {
	status := &HealthStatus{
		Liveness:  "ok",
		Readiness: "ok",
		Services:  make(map[string]string),
	}

	for _, mp := range s.procs {
		mp.mu.Lock()
		if mp.running {
			status.Services[mp.name] = "running"
		} else {
			status.Services[mp.name] = "stopped"
			status.Readiness = "degraded"
		}
		mp.mu.Unlock()
	}

	if s.caddy != nil {
		status.CaddyRunning = s.caddy.IsRunning()
		if !status.CaddyRunning {
			status.Readiness = "degraded"
		}
	}

	if s.litestream != nil {
		lag := s.litestream.Lag()
		status.LitestreamLag = lag
		if lag > 10 {
			status.Readiness = "degraded"
		}
	}

	if len(s.procs) == 0 {
		status.Liveness = "degraded"
	}

	return status
}

func (s *Supervisor) ProcessCount() int {
	return len(s.procs)
}

func buildCommand(cmd string) *exec.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		return exec.Command(parts[0])
	}
	return exec.Command(parts[0], parts[1:]...)
}
