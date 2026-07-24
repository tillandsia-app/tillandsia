package types

type ServiceType string

const (
	ServiceTypeWeb    ServiceType = "web"
	ServiceTypeWorker ServiceType = "worker"
	ServiceTypeCron   ServiceType = "cron"
)

type Service struct {
	Type    ServiceType `yaml:"-" json:"type"`
	Command string      `yaml:"-" json:"command"`
}

type BuildConfig struct {
	Dockerfile string `yaml:"dockerfile" json:"dockerfile"`
	Context    string `yaml:"context" json:"context"`
}

type Config struct {
	Name  string            `yaml:"name" json:"name"`
	Port  int               `yaml:"port" json:"port"`
	Env   map[string]string `yaml:"env" json:"env"`
	Build BuildConfig       `yaml:"build" json:"build"`
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
	Name     string
	Dir      string
	Config   *Config
	Services []*Service
}