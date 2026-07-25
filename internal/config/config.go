package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tillandsia-app/tillandsia/internal/types"
)

const ConfigFileName = "tillandsia.yaml"

func FindConfigDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		path := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(path); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found in any parent directory", ConfigFileName)
		}
		dir = parent
	}
}

func ReadConfig(dir string) (*types.Config, error) {
	path := filepath.Join(dir, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigFileName, err)
	}
	cfg := &types.Config{
		Port: 8080,
		Env:  make(map[string]string),
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigFileName, err)
	}
	if cfg.Name == "" {
		cfg.Name = filepath.Base(dir)
	}
	if err := types.ValidateAppName(cfg.Name); err != nil {
		return nil, fmt.Errorf("%s: %w", ConfigFileName, err)
	}
	if cfg.Services == nil {
		return nil, fmt.Errorf("%s: at least one service is required (e.g. 'web: node server.js')", ConfigFileName)
	}
	for st := range cfg.Services {
		if !types.ValidServiceType(st) {
			return nil, fmt.Errorf("%s: invalid service type %q (must be web, worker, or cron)", ConfigFileName, st)
		}
	}
	if cfg.Build != nil && cfg.Build.Context == "" {
		cfg.Build.Context = "."
	}
	if cfg.Build != nil && cfg.Build.Dockerfile == "" {
		cfg.Build.Dockerfile = "Dockerfile"
	}
	return cfg, nil
}

func WriteConfig(dir string, cfg *types.Config) error {
	path := filepath.Join(dir, ConfigFileName)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func ReadServersConfig() (*types.ServersConfig, error) {
	path, err := serversConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &types.ServersConfig{Servers: make(map[string]*types.Server)}, nil
		}
		return nil, fmt.Errorf("reading servers config: %w", err)
	}
	sc := &types.ServersConfig{}
	if err := yaml.Unmarshal(data, sc); err != nil {
		return nil, fmt.Errorf("parsing servers config: %w", err)
	}
	if sc.Servers == nil {
		sc.Servers = make(map[string]*types.Server)
	}
	return sc, nil
}

func WriteServersConfig(sc *types.ServersConfig) error {
	path, err := serversConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshaling servers config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func serversConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tillandsia", "servers.yaml"), nil
}

func ServicesFromConfig(cfg *types.Config) string {
	var lines []string
	for _, k := range sortedKeys(cfg.Services) {
		lines = append(lines, fmt.Sprintf("%s: %s", k, cfg.Services[k]))
	}
	return strings.Join(lines, "\n")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}