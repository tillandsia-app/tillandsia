package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/tillandsia/tillandsia/internal/types"
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
		Build: types.BuildConfig{
			Dockerfile: "Dockerfile",
			Context:    ".",
		},
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ConfigFileName, err)
	}
	if cfg.Name == "" {
		cfg.Name = filepath.Base(dir)
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
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

const ProcfileName = "Procfile"

func ReadProcfile(dir string) ([]*types.Service, error) {
	path := filepath.Join(dir, ProcfileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", ProcfileName, err)
	}
	return ParseProcfile(string(data))
}

func ParseProcfile(content string) ([]*types.Service, error) {
	var services []*types.Service
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid Procfile line: %q", line)
		}
		st := strings.TrimSpace(parts[0])
		cmd := strings.TrimSpace(parts[1])
		svc := &types.Service{
			Type:    types.ServiceType(st),
			Command: cmd,
		}
		switch svc.Type {
		case types.ServiceTypeWeb, types.ServiceTypeWorker, types.ServiceTypeCron:
		default:
			return nil, fmt.Errorf("unknown service type %q in Procfile line %q", svc.Type, line)
		}
		services = append(services, svc)
	}
	return services, nil
}

func WriteProcfile(dir string, services []*types.Service) error {
	var lines []string
	for _, svc := range services {
		lines = append(lines, fmt.Sprintf("%s: %s", svc.Type, svc.Command))
	}
	path := filepath.Join(dir, ProcfileName)
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
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