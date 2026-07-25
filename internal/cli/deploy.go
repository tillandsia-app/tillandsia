package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tillandsia-app/tillandsia/internal/config"
	"github.com/tillandsia-app/tillandsia/internal/deploy"
	"github.com/tillandsia-app/tillandsia/internal/ssh"
)

func init() {
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(statusCmd)

	deployCmd.Flags().String("server", "", "Target server name")
	deployCmd.Flags().Bool("dry-run", false, "Show the deploy plan without executing")
	deployCmd.Flags().Bool("rollback", false, "Re-deploy the previous image")

	statusCmd.Flags().String("server", "", "Target server name")
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your application",
	Long: `Build, wrap, transfer, and run your application on a server.

The deploy pipeline:
  1. Builds a Docker image from your runtime or custom Dockerfile
  2. Wraps it with the Tillandsia init system
  3. Transfers the image to the server via SSH
  4. Stops the old container
  5. Starts the new container
  6. Waits for the health check

Examples:
  tillandsia deploy
  tillandsia deploy --server my-vps
  tillandsia deploy --dry-run
  tillandsia deploy --rollback
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		rollback, _ := cmd.Flags().GetBool("rollback")

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

		opts := &deploy.Options{
			AppDir:   cfgDir,
			AppName:  cfg.Name,
			Server:   srv,
			Config:   cfg,
			Rollback: rollback,
			DryRun:   dryRun,
		}

		if dryRun {
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"app":    cfg.Name,
					"server": srv.Host,
					"steps":  []string{"build", "wrap", "transfer", "stop", "start", "health"},
				})
			}
			fmt.Printf("Dry-run: would deploy %q to %s@%s\n", cfg.Name, srv.User, srv.Host)
			fmt.Println("  Steps:")
			fmt.Println("    1. Build user image")
			fmt.Println("    2. Wrap with init system")
			fmt.Println("    3. Transfer image via SSH")
			fmt.Println("    4. Stop old container")
			fmt.Println("    5. Start new container")
			fmt.Println("    6. Wait for health check")
			return nil
		}

		fmt.Fprintf(os.Stderr, "Deploying %q to %s@%s...\n", cfg.Name, srv.User, srv.Host)

		result, err := deploy.Run(context.Background(), opts)
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}

		fmt.Fprintf(os.Stderr, "Deploy of %q to %s complete.\n", cfg.Name, result.ServerHost)
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status",
	Long: `Show the current status of a deployed application.

Queries the server for container state, health, and uptime.

Examples:
  tillandsia status
  tillandsia status --server my-vps
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")

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

		inspectCmd := fmt.Sprintf(
			`docker inspect %s --format '{"id":"{{.ID}}","status":"{{.State.Status}}","health":"{{.State.Health.Status}}","started":"{{.State.StartedAt}}","image":"{{.Config.Image}}"}' 2>/dev/null || echo '{"status":"not-found"}'`,
			cfg.Name,
		)

		result, err := client.Run(context.Background(), inspectCmd)
		if err != nil {
			return fmt.Errorf("inspecting container: %w", err)
		}

		if jsonOutput {
			fmt.Println(result.Stdout)
			return nil
		}

		var info struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Health  string `json:"health"`
			Started string `json:"started"`
			Image   string `json:"image"`
		}
		if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing container info: %v\n", err)
			fmt.Printf("Container: %s\n", cfg.Name)
			fmt.Printf("Status:    unknown\n")
			return nil
		}

		if info.Status == "not-found" {
			fmt.Printf("Container %q is not running on %s.\n", cfg.Name, srv.Host)
			return nil
		}

		healthStatus := info.Status
		if info.Health != "" && info.Health != "<nil>" {
			healthStatus = info.Health
		}

		idShort := info.ID
		if len(idShort) > 12 {
			idShort = idShort[:12]
		}

		fmt.Printf("Container: %s\n", cfg.Name)
		fmt.Printf("ID:        %s\n", idShort)
		fmt.Printf("Status:    %s\n", healthStatus)
		fmt.Printf("Image:     %s\n", info.Image)
		fmt.Printf("Server:    %s (%s@%s)\n", srv.Host, srv.User, srv.Host)
		if info.Started != "" {
			fmt.Printf("Started:   %s\n", info.Started)
		}

		return nil
	},
}

func sortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func envMapToSortedSlice(m map[string]string) []map[string]string {
	var items []map[string]string
	for _, k := range sortedKeys(m) {
		items = append(items, map[string]string{"key": k, "value": m[k]})
	}
	return items
}