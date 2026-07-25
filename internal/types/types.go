package types

import (
	"fmt"
	"regexp"
)

type ServiceType string

const (
	ServiceTypeWeb    ServiceType = "web"
	ServiceTypeWorker ServiceType = "worker"
	ServiceTypeCron   ServiceType = "cron"
)

var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func ValidateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name must not be empty")
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("invalid app name %q: only letters, digits, hyphens, and underscores allowed", name)
	}
	if len(name) > 64 {
		return fmt.Errorf("app name too long (max 64 characters)")
	}
	return nil
}

type BuildConfig struct {
	Dockerfile string `yaml:"dockerfile" json:"dockerfile"`
	Context    string `yaml:"context" json:"context"`
}

type Config struct {
	Name     string            `yaml:"name" json:"name"`
	Port     int               `yaml:"port" json:"port"`
	Runtime  string            `yaml:"runtime" json:"runtime"`
	Services map[string]string `yaml:"services" json:"services"`
	Env      map[string]string `yaml:"env" json:"env"`
	Build    *BuildConfig      `yaml:"build,omitempty" json:"build,omitempty"`
}

type LitestreamConfig struct {
	AccessKeyID     string `yaml:"access-key-id" json:"access-key-id"`
	SecretAccessKey string `yaml:"secret-access-key" json:"secret-access-key"`
	URL             string `yaml:"url" json:"url"`
	Region          string `yaml:"region" json:"region"`
}

type Server struct {
	Host       string            `yaml:"host" json:"host"`
	User       string            `yaml:"user" json:"user"`
	Key        string            `yaml:"key" json:"key"`
	Port       int               `yaml:"port" json:"port"`
	Default    bool              `yaml:"default" json:"default"`
	Provider   string            `yaml:"provider,omitempty" json:"provider,omitempty"`
	ProviderID string            `yaml:"provider_id,omitempty" json:"provider_id,omitempty"`
	Litestream *LitestreamConfig `yaml:"litestream,omitempty" json:"litestream,omitempty"`
	Domain     string            `yaml:"domain,omitempty" json:"domain,omitempty"`
}

type ServersConfig struct {
	Servers map[string]*Server `yaml:"servers" json:"servers"`
}

type EnvEntry struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type App struct {
	Name   string
	Dir    string
	Config *Config
}

func ValidServiceType(s string) bool {
	switch ServiceType(s) {
	case ServiceTypeWeb, ServiceTypeWorker, ServiceTypeCron:
		return true
	default:
		return false
	}
}