package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/caddyserver/caddy/v2"
	caddyhttp "github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

const CaddyfilePath = "/etc/caddy/Caddyfile"

type CaddyManager struct {
	domain      string
	port        int
	customCaddy string
	running     bool
	logger      *log.Logger
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
	return os.WriteFile(CaddyfilePath, []byte(content), 0644)
}

func (cm *CaddyManager) buildConfig() (*caddy.Config, error) {
	route := caddyhttp.Route{
		HandlersRaw: []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`{"handler": "subroute", "routes": [{"handle": [{"handler": "reverse_proxy", "upstreams": [{"dial": "localhost:%d"}]}]}]}`, cm.port)),
		},
	}

	if cm.domain != "" {
		route.MatcherSetsRaw = caddyhttp.RawMatcherSets{
			caddy.ModuleMap{
				"host": json.RawMessage(fmt.Sprintf(`["%s"]`, cm.domain)),
			},
		}
	}

	listenAddr := ":80"
	if cm.domain != "" {
		listenAddr = fmt.Sprintf(":%d", 443)
	}

	server := &caddyhttp.Server{
		Listen: []string{listenAddr},
		Routes: caddyhttp.RouteList{route},
	}

	httpApp := &caddyhttp.App{
		Servers: map[string]*caddyhttp.Server{"tillandsia": server},
	}

	cfg := &caddy.Config{
		AppsRaw: caddy.ModuleMap{
			"http": json.RawMessage(`{}`),
		},
		Admin: &caddy.AdminConfig{
			Disabled: true,
		},
	}

	httpAppRaw, err := json.Marshal(httpApp)
	if err != nil {
		return nil, fmt.Errorf("marshaling http app config: %w", err)
	}
	cfg.AppsRaw["http"] = httpAppRaw

	return cfg, nil
}

func (cm *CaddyManager) Start(ctx context.Context) error {
	cfg, err := cm.buildConfig()
	if err != nil {
		return fmt.Errorf("building caddy config: %w", err)
	}

	if err := caddy.Run(cfg); err != nil {
		return fmt.Errorf("starting caddy: %w", err)
	}

	cm.running = true
	cm.logger.Println("caddy started")

	// Monitor for exit in background
	go func() {
		<-ctx.Done()
		cm.Stop(context.Background())
	}()

	return nil
}

func (cm *CaddyManager) Stop(ctx context.Context) {
	if cm.running {
		cm.logger.Println("stopping caddy...")
		if err := caddy.Stop(); err != nil {
			cm.logger.Printf("caddy stop error: %v", err)
		}
		cm.running = false
		cm.logger.Println("caddy stopped")
	}
}

func (cm *CaddyManager) IsRunning() bool {
	return cm.running
}

func ZeroDowntimeHandoff(ctx context.Context, mgmtPort int, appName string) error {
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