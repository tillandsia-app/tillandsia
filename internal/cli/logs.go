package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/tillandsia-app/tillandsia/internal/config"
	"github.com/tillandsia-app/tillandsia/internal/deploy"
	"github.com/tillandsia-app/tillandsia/internal/ssh"
)

// TODO: Consider a local CLI command that serves basic analytics from the
// container through a proxy (geo stamping by IP, most visited pages, etc.)
// so it's not exposed to the public internet.

func init() {
	rootCmd.AddCommand(logsCmd)

	logsCmd.Flags().String("server", "", "Target server name")
	logsCmd.Flags().Int("tail", 50, "Number of lines to show from the end of logs")
	logsCmd.Flags().BoolP("follow", "f", true, "Follow log output")
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream application logs",
	Long: `Show and follow log output from the deployed application.

Connects to the server and streams container logs in real-time.

Examples:
  tillandsia logs
  tillandsia logs --tail 100
  tillandsia logs --no-follow
  tillandsia logs --server my-vps
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		tailLines, _ := cmd.Flags().GetInt("tail")
		follow, _ := cmd.Flags().GetBool("follow")

		cfgDir, err := config.FindConfigDir()
		if err != nil {
			return err
		}

		cfg, err := config.ReadConfig(cfgDir)
		if err != nil {
			return err
		}

		srv, err := deploy.GetServerConfig(serverName)
		if err != nil {
			return err
		}

		client, err := ssh.Connect(srv.Host, srv.User, srv.Key, srv.Port)
		if err != nil {
			return fmt.Errorf("connecting to server: %w", err)
		}
		defer client.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			<-sigCh
			cancel()
		}()

		logCmd := fmt.Sprintf("docker logs --tail %d", tailLines)
		if follow {
			logCmd += " --follow"
		}
		logCmd += " " + cfg.Name

		if jsonOutput {
			if err := client.RunStream(ctx, logCmd+" 2>&1", os.Stdout, os.Stderr); err != nil {
				return err
			}
			return nil
		}

		fmt.Fprintf(os.Stderr, "Streaming logs for %q on %s...\n", cfg.Name, srv.Host)
		if err := client.RunStream(ctx, logCmd, os.Stdout, os.Stderr); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Try combined output if stdout-only produced nothing
			if follow {
				return client.RunStream(ctx, logCmd+" 2>&1", os.Stdout, os.Stderr)
			}
			return err
		}
		return nil
	},
}

func resolveServer(name string) (*configResolvedServer, error) {
	sc, err := config.ReadServersConfig()
	if err != nil {
		return nil, err
	}
	if name != "" {
		s, ok := sc.Servers[name]
		if !ok {
			return nil, fmt.Errorf("server %q not found", name)
		}
		return &configResolvedServer{Host: s.Host, User: s.User, Key: s.Key, Port: s.Port}, nil
	}
	for _, s := range sc.Servers {
		if s.Default {
			return &configResolvedServer{Host: s.Host, User: s.User, Key: s.Key, Port: s.Port}, nil
		}
	}
	return nil, fmt.Errorf("no server specified and no default server set")
}

type configResolvedServer struct {
	Host string
	User string
	Key  string
	Port int
}

func jsonOrPrint(entry interface{}) error {
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(entry)
	}
	fmt.Println(entry)
	return nil
}