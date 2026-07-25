package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tillandsia-app/tillandsia/internal/config"
	"github.com/tillandsia-app/tillandsia/internal/supervisor"
)

func main() {
	ctx := context.Background()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading config: %v\n", err)
		os.Exit(1)
	}

	supervisor := supervisor.NewSupervisor(cfg)

	if err := supervisor.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() (*supervisor.InitConfig, error) {
	// Try to read tillandsia.yaml from /app or current directory
	searchDirs := []string{"/app", "."}
	var appCfg *configFromFile
	for _, dir := range searchDirs {
		path := filepath.Join(dir, config.ConfigFileName)
		if _, err := os.Stat(path); err == nil {
			tc, err := config.ReadConfig(dir)
			if err != nil {
				return nil, fmt.Errorf("reading config from %s: %w", dir, err)
			}
			appCfg = &configFromFile{
				Name:     tc.Name,
				Port:     tc.Port,
				Runtime:  tc.Runtime,
				Services: tc.Services,
				Env:      tc.Env,
			}
			break
		}
	}

	ic := &supervisor.InitConfig{
		Name:    getEnv("TILLANDSIA_APP_NAME", ""),
		Port:    getEnvInt("PORT", 8080),
		Domain:  getEnv("TILLANDSIA_DOMAIN", ""),
		Services: make(map[string]string),
		Env:     make(map[string]string),
	}

	// Merge from file config if found
	if appCfg != nil {
		if ic.Name == "" {
			ic.Name = appCfg.Name
		}
		if ic.Port == 0 && appCfg.Port != 0 {
			ic.Port = appCfg.Port
		}
		for k, v := range appCfg.Services {
			ic.Services[k] = v
		}
		for k, v := range appCfg.Env {
			ic.Env[k] = v
		}
	}

	// Parse services from env var (overrides file config)
	servicesEnv := getEnv("TILLANDSIA_SERVICES", "")
	if servicesEnv != "" {
		ic.Services = make(map[string]string)
		for _, part := range strings.Split(servicesEnv, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 {
				ic.Services[kv[0]] = kv[1]
			}
		}
	}

	// Parse env from TILLANDSIA_ENV env var
	envEnv := getEnv("TILLANDSIA_ENV", "")
	if envEnv != "" {
		for _, part := range strings.Split(envEnv, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 {
				ic.Env[kv[0]] = kv[1]
			}
		}
	}

	// Merge PORT into env
	ic.Env["PORT"] = strconv.Itoa(ic.Port)

	// Litestream config from env
	lsURL := getEnv("LITESTREAM_URL", "")
	if lsURL != "" {
		ic.Litestream = &supervisor.LitestreamEnv{
			AccessKeyID:     getEnv("LITESTREAM_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("LITESTREAM_SECRET_ACCESS_KEY", ""),
			URL:             lsURL,
			Region:          getEnv("LITESTREAM_REGION", ""),
		}
	}

	// Custom Caddyfile
	ic.CustomCaddy = getEnv("TILLANDSIA_CADDYFILE", "")

	// Retry limit
	ic.RetryLimit = getEnvInt("TILLANDSIA_RETRY_LIMIT", 3)

	// Health path
	ic.HealthPath = getEnv("TILLANDSIA_HEALTH_PATH", "/")

	if ic.Name == "" {
		ic.Name = "app"
	}

	return ic, nil
}

type configFromFile struct {
	Name     string
	Port     int
	Runtime  string
	Services map[string]string
	Env      map[string]string
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}