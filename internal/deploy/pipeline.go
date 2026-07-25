package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tillandsia-app/tillandsia/internal/build"
	"github.com/tillandsia-app/tillandsia/internal/config"
	"github.com/tillandsia-app/tillandsia/internal/ssh"
	"github.com/tillandsia-app/tillandsia/internal/types"
)

type Options struct {
	AppDir              string
	AppName             string
	Server              *types.Server
	Config              *types.Config
	Rollback            bool
	DryRun              bool
	HealthCheckInterval time.Duration // poll interval, default 5s
}

type Result struct {
	ImageTag   string
	ServerHost string
}

type Step interface {
	Name() string
	Run(ctx context.Context) error
}

type StepFunc func(ctx context.Context, name string, fn func() error) error

var DefaultStepFunc StepFunc = func(ctx context.Context, name string, fn func() error) error {
	fmt.Fprintf(os.Stderr, "  %s... ", name)
	if err := fn(); err != nil {
		fmt.Fprintln(os.Stderr, "FAILED")
		return fmt.Errorf("%s: %w", name, err)
	}
	fmt.Fprintln(os.Stderr, "OK")
	return nil
}

func Run(ctx context.Context, opts *Options) (*Result, error) {
	return RunWithSteps(ctx, opts, DefaultStepFunc)
}

type buildUserImageStep struct {
	opts *Options
	tag  string
}

func (s *buildUserImageStep) Name() string { return "Building user image" }
func (s *buildUserImageStep) Run(ctx context.Context) error {
	return buildUserImage(ctx, s.opts, s.tag)
}

type buildWrapperImageStep struct {
	opts       *Options
	baseTag    string
	wrappedTag string
}

func (s *buildWrapperImageStep) Name() string { return "Wrapping with init system" }
func (s *buildWrapperImageStep) Run(ctx context.Context) error {
	return buildWrapperImage(ctx, s.opts, s.baseTag, s.wrappedTag)
}

type transferImageStep struct {
	client *ssh.Client
	tag    string
}

func (s *transferImageStep) Name() string { return "Transferring image" }
func (s *transferImageStep) Run(ctx context.Context) error {
	return transferImage(ctx, s.client, s.tag)
}

type stopContainerStep struct {
	client  *ssh.Client
	appName string
}

func (s *stopContainerStep) Name() string { return "Stopping old container" }
func (s *stopContainerStep) Run(ctx context.Context) error {
	return stopContainer(ctx, s.client, s.appName)
}

type startContainerStep struct {
	client  *ssh.Client
	opts    *Options
	tag     string
	dataDir string
}

func (s *startContainerStep) Name() string { return "Starting container" }
func (s *startContainerStep) Run(ctx context.Context) error {
	return startContainer(ctx, s.client, s.opts, s.tag, s.dataDir)
}

type cleanupDataDirsStep struct {
	client  *ssh.Client
	appName string
}

func (s *cleanupDataDirsStep) Name() string { return "Cleaning up old data directories" }
func (s *cleanupDataDirsStep) Run(ctx context.Context) error {
	return cleanupDataDirs(ctx, s.client, s.appName)
}

type waitForHealthyStep struct {
	client   *ssh.Client
	appName  string
	interval time.Duration
}

func (s *waitForHealthyStep) Name() string { return "Waiting for health" }
func (s *waitForHealthyStep) Run(ctx context.Context) error {
	return waitForHealthy(ctx, s.client, s.appName, s.interval)
}

func RunWithSteps(ctx context.Context, opts *Options, stepFn StepFunc) (*Result, error) {
	if err := types.ValidateAppName(opts.AppName); err != nil {
		return nil, fmt.Errorf("invalid app name: %w", err)
	}

	client, err := ssh.Connect(opts.Server.Host, opts.Server.User, opts.Server.Key, opts.Server.Port)
	if err != nil {
		return nil, fmt.Errorf("connecting to server: %w", err)
	}
	defer client.Close()

	if opts.Rollback {
		return runRollback(ctx, client, opts, stepFn)
	}
	return runDeploy(ctx, client, opts, stepFn)
}

func runDeploy(ctx context.Context, client *ssh.Client, opts *Options, stepFn StepFunc) (*Result, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	userTag := fmt.Sprintf("tillandsia/%s:%s", opts.AppName, timestamp)
	wrappedTag := userTag + "-wrapped"
	dataDir := fmt.Sprintf("/var/lib/tillandsia/%s/data-%s", opts.AppName, timestamp)

	steps := []Step{
		&buildUserImageStep{opts: opts, tag: userTag},
		&buildWrapperImageStep{opts: opts, baseTag: userTag, wrappedTag: wrappedTag},
		&transferImageStep{client: client, tag: wrappedTag},
		&stopContainerStep{client: client, appName: opts.AppName},
		&startContainerStep{client: client, opts: opts, tag: wrappedTag, dataDir: dataDir},
		&waitForHealthyStep{client: client, appName: opts.AppName, interval: opts.HealthCheckInterval},
		&cleanupDataDirsStep{client: client, appName: opts.AppName},
	}

	for _, s := range steps {
		s := s
		if err := stepFn(ctx, s.Name(), func() error { return s.Run(ctx) }); err != nil {
			return nil, err
		}
	}

	return &Result{ImageTag: wrappedTag, ServerHost: opts.Server.Host}, nil
}

func runRollback(ctx context.Context, client *ssh.Client, opts *Options, stepFn StepFunc) (*Result, error) {
	prevTag, err := findPreviousImage(ctx, client, opts.AppName)
	if err != nil {
		return nil, err
	}
	wrappedTag := fmt.Sprintf("tillandsia/%s:%s-wrapped", opts.AppName, prevTag)
	dataDir := fmt.Sprintf("/var/lib/tillandsia/%s/data-%s", opts.AppName, prevTag)

	steps := []Step{
		&stopContainerStep{client: client, appName: opts.AppName},
		&startContainerStep{client: client, opts: opts, tag: wrappedTag, dataDir: dataDir},
		&waitForHealthyStep{client: client, appName: opts.AppName, interval: opts.HealthCheckInterval},
	}

	for _, s := range steps {
		s := s
		if err := stepFn(ctx, s.Name(), func() error { return s.Run(ctx) }); err != nil {
			return nil, err
		}
	}

	return &Result{ImageTag: wrappedTag, ServerHost: opts.Server.Host}, nil
}

func buildUserImage(ctx context.Context, opts *Options, tag string) error {
	dfPath := filepath.Join(opts.AppDir, "Dockerfile")
	hasCustomDockerfile := false
	if _, err := os.Stat(dfPath); err == nil {
		hasCustomDockerfile = true
	}

	buildCtx := opts.AppDir
	dfArg := dfPath

	if !hasCustomDockerfile {
		webCmd := ""
		if c, ok := opts.Config.Services["web"]; ok {
			webCmd = c
		} else {
			for _, cmd := range opts.Config.Services {
				webCmd = cmd
				break
			}
		}

		dfContent, err := build.GenerateDockerfile(opts.Config.Runtime, opts.AppDir, webCmd)
		if err != nil {
			return fmt.Errorf("generating Dockerfile: %w", err)
		}

		tmpDf := filepath.Join(opts.AppDir, ".tillandsia-Dockerfile")
		if err := os.WriteFile(tmpDf, []byte(dfContent), 0644); err != nil {
			return fmt.Errorf("writing temporary Dockerfile: %w", err)
		}
		defer os.Remove(tmpDf)

		dfArg = tmpDf
	}

	args := []string{"build", "-t", tag, "-f", dfArg}
	if opts.Config.Build != nil && opts.Config.Build.Context != "" {
		buildCtx = opts.Config.Build.Context
	}
	args = append(args, buildCtx)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildWrapperImage(ctx context.Context, opts *Options, baseTag, wrappedTag string) error {
	tmpDir, err := os.MkdirTemp("", "tillandsia-wrap-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	initBinary := filepath.Join(tmpDir, "tillandsia-init")
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", initBinary, "./cmd/tillandsia-init")
	buildCmd.Dir = opts.AppDir
	buildCmd.Stdout = os.Stderr
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		buildCmd = exec.CommandContext(ctx, "go", "build", "-o", initBinary,
			"github.com/tillandsia-app/tillandsia/cmd/tillandsia-init")
		buildCmd.Stdout = os.Stderr
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("building tillandsia-init: %w", err)
		}
	}

	dfContent := fmt.Sprintf(`FROM %s
COPY tillandsia-init /usr/local/bin/tillandsia-init
RUN chmod +x /usr/local/bin/tillandsia-init
ENTRYPOINT ["tillandsia-init"]
`, baseTag)

	dfPath := filepath.Join(tmpDir, "Dockerfile.wrapper")
	if err := os.WriteFile(dfPath, []byte(dfContent), 0644); err != nil {
		return fmt.Errorf("writing wrapper Dockerfile: %w", err)
	}

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", wrappedTag, "-f", dfPath, tmpDir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func transferImage(ctx context.Context, client *ssh.Client, tag string) error {
	saveCmd := exec.CommandContext(ctx, "docker", "save", tag)
	pipe, err := saveCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	saveCmd.Stderr = os.Stderr

	if err := saveCmd.Start(); err != nil {
		return fmt.Errorf("starting docker save: %w", err)
	}

	if err := client.PipeToCommand(ctx, pipe, "docker load"); err != nil {
		saveCmd.Process.Kill()
		return fmt.Errorf("transferring image: %w", err)
	}

	return saveCmd.Wait()
}

func stopContainer(ctx context.Context, client *ssh.Client, appName string) error {
	result, err := client.Run(ctx, fmt.Sprintf(
		"docker inspect --format '{{.State.Status}}' %s 2>/dev/null || echo not-found", appName))
	if err != nil {
		return err
	}
	status := strings.TrimSpace(result.Stdout)
	if status == "not-found" || status == "" {
		return nil
	}

	cmds := []string{
		fmt.Sprintf("docker stop --time 30 %s 2>/dev/null || true", appName),
		fmt.Sprintf("docker rm %s 2>/dev/null || true", appName),
	}
	for _, c := range cmds {
		if _, err := client.Run(ctx, c); err != nil {
			return fmt.Errorf("stopping container: %w", err)
		}
	}
	return nil
}

func startContainer(ctx context.Context, client *ssh.Client, opts *Options, tag string, dataDir string) error {
	envFile := fmt.Sprintf("/var/lib/tillandsia/%s/env", opts.AppName)

	port := opts.Config.Port
	if port == 0 {
		port = 8080
	}

	envVars := []string{
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("TILLANDSIA_APP_NAME=%s", opts.AppName),
		fmt.Sprintf("TILLANDSIA_SERVICES=%s", servicesEnvStr(opts.Config.Services)),
	}
	for k, v := range opts.Config.Env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	if opts.Server.Litestream != nil {
		envVars = append(envVars,
			fmt.Sprintf("LITESTREAM_URL=%s", opts.Server.Litestream.URL),
			fmt.Sprintf("LITESTREAM_ACCESS_KEY_ID=%s", opts.Server.Litestream.AccessKeyID),
			fmt.Sprintf("LITESTREAM_SECRET_ACCESS_KEY=%s", opts.Server.Litestream.SecretAccessKey),
			fmt.Sprintf("LITESTREAM_REGION=%s", opts.Server.Litestream.Region),
		)
	}
	if opts.Server.Domain != "" {
		envVars = append(envVars, fmt.Sprintf("TILLANDSIA_DOMAIN=%s", opts.Server.Domain))
	}

	envContent := strings.Join(envVars, "\n")

	if err := client.PipeToCommand(ctx,
		strings.NewReader(envContent),
		fmt.Sprintf("mkdir -p /var/lib/tillandsia/%s %s && cat > %s", opts.AppName, dataDir, envFile)); err != nil {
		return fmt.Errorf("writing env file: %w", err)
	}

	runArgs := []string{
		"run", "-d",
		"--name", opts.AppName,
		"--restart", "unless-stopped",
		"-v", fmt.Sprintf("%s:/data", dataDir),
		"-v", fmt.Sprintf("%s:/run/tillandsia/env:ro", envFile),
	}
	for _, e := range envVars {
		runArgs = append(runArgs, "-e", e)
	}
	runArgs = append(runArgs, "-p", "80:80", "-p", "443:443", tag)

	result, err := client.Run(ctx, fmt.Sprintf("docker %s", strings.Join(runArgs, " ")))
	if err != nil {
		return fmt.Errorf("starting container: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("starting container failed:\n%s", result.Stderr)
	}
	return nil
}

func waitForHealthy(ctx context.Context, client *ssh.Client, appName string, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timed out")
		case <-ticker.C:
			result, err := client.Run(ctx, fmt.Sprintf(
				"docker inspect --format '{{.State.Health.Status}}' %s 2>/dev/null || docker inspect --format '{{.State.Status}}' %s 2>/dev/null || echo not-found",
				appName, appName))
			if err != nil {
				continue
			}
			status := strings.TrimSpace(result.Stdout)
			if status == "healthy" || status == "running" {
				return nil
			}
			fmt.Fprintf(os.Stderr, "  waiting for container (status: %s)...\n", status)
		}
	}
}

func findPreviousImage(ctx context.Context, client *ssh.Client, appName string) (string, error) {
	result, err := client.Run(ctx, fmt.Sprintf(
		"docker images tillandsia/%s --format '{{.Tag}}' | grep -v wrapped | sort -n | tail -2 | head -1",
		appName))
	if err != nil {
		return "", err
	}
	prevTag := strings.TrimSpace(result.Stdout)
	if prevTag == "" {
		return "", fmt.Errorf("no previous image found for rollback")
	}
	return prevTag, nil
}

func cleanupDataDirs(ctx context.Context, client *ssh.Client, appName string) error {
	basePath := fmt.Sprintf("/var/lib/tillandsia/%s", appName)
	// List data-* dirs, keep last 3, remove older ones
	cmd := fmt.Sprintf(
		"ls -1d %s/data-* 2>/dev/null | sort | head -n -3 | xargs -r rm -rf", basePath)
	_, err := client.Run(ctx, cmd)
	return err
}

func servicesEnvStr(services map[string]string) string {
	var parts []string
	for k, v := range services {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

func GetServerConfig(name string) (*types.Server, error) {
	sc, err := config.ReadServersConfig()
	if err != nil {
		return nil, fmt.Errorf("reading servers: %w", err)
	}
	if name != "" {
		srv, ok := sc.Servers[name]
		if !ok {
			return nil, fmt.Errorf("server %q not found", name)
		}
		return srv, nil
	}
	for _, s := range sc.Servers {
		if s.Default {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no default server set and no server name provided")
}

func EnvFilePath(appName string) string {
	return fmt.Sprintf("/var/lib/tillandsia/%s/env", appName)
}

func ReadEnvFile(ctx context.Context, client *ssh.Client, appName string) (map[string]string, error) {
	env := make(map[string]string)
	result, err := client.Run(ctx, fmt.Sprintf("cat %s 2>/dev/null || true", EnvFilePath(appName)))
	if err != nil {
		return env, err
	}
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env, nil
}

func WriteEnvFile(ctx context.Context, client *ssh.Client, appName string, env map[string]string) error {
	var lines []string
	for k, v := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}
	content := strings.Join(lines, "\n")

	return client.PipeToCommand(ctx,
		strings.NewReader(content),
		fmt.Sprintf("mkdir -p /var/lib/tillandsia/%s && cat > %s", appName, EnvFilePath(appName)))
}

func RestartContainer(ctx context.Context, client *ssh.Client, appName string) error {
	result, err := client.Run(ctx, fmt.Sprintf("docker restart %s", appName))
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("restarting container: %s", result.Stderr)
	}
	return nil
}