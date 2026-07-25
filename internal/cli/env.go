package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tillandsia-app/tillandsia/internal/config"
	"github.com/tillandsia-app/tillandsia/internal/deploy"
	"github.com/tillandsia-app/tillandsia/internal/ssh"
)

func init() {
	rootCmd.AddCommand(envCmd)

	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envGetCmd)
	envCmd.AddCommand(envRmCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envExportCmd)

	envSetCmd.Flags().String("server", "", "Target server name")
	envGetCmd.Flags().String("server", "", "Target server name")
	envRmCmd.Flags().String("server", "", "Target server name")
	envListCmd.Flags().String("server", "", "Target server name")
	envExportCmd.Flags().String("server", "", "Target server name")
}

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"environment"},
	Short:   "Manage environment variables",
	Long:    `Set, get, list, remove, and export environment variables for a deployed application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set <key>=<value> [<key>=<value>...]",
	Short: "Set environment variables",
	Long: `Set one or more environment variables and restart the container.

Each argument must be in KEY=VALUE format.

Examples:
  tillandsia env set NODE_ENV=production
  tillandsia env set DATABASE_URL=postgres://... REDIS_URL=redis://...
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		appName, srv, client, err := connectForEnv(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		env, err := deploy.ReadEnvFile(context.Background(), client, appName)
		if err != nil {
			return fmt.Errorf("reading env file: %w", err)
		}

		for _, arg := range args {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid format %q, expected KEY=VALUE", arg)
			}
			env[parts[0]] = parts[1]
		}

		if err := writeEnvAndRestart(client, appName, env); err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"status":  "updated",
				"app":     appName,
				"server":  srv.Host,
				"updated": len(args),
			})
		}
		fmt.Fprintf(os.Stderr, "Set %d env var(s) and restarted %q on %s.\n", len(args), appName, srv.Host)
		return nil
	},
}

var envGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get an environment variable",
	Long: `Get the value of a specific environment variable.

Example:
  tillandsia env get NODE_ENV
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		appName, _, client, err := connectForEnv(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		env, err := deploy.ReadEnvFile(context.Background(), client, appName)
		if err != nil {
			return fmt.Errorf("reading env file: %w", err)
		}

		key := args[0]
		val, ok := env[key]
		if !ok {
			return fmt.Errorf("env var %q not found", key)
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"key": key, "value": val})
		}
		fmt.Println(val)
		return nil
	},
}

var envRmCmd = &cobra.Command{
	Use:   "rm <key> [<key>...]",
	Short: "Remove environment variables",
	Long: `Remove one or more environment variables and restart the container.

Examples:
  tillandsia env rm NODE_ENV
  tillandsia env rm DATABASE_URL REDIS_URL
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		appName, srv, client, err := connectForEnv(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		env, err := deploy.ReadEnvFile(context.Background(), client, appName)
		if err != nil {
			return fmt.Errorf("reading env file: %w", err)
		}

		for _, key := range args {
			delete(env, key)
		}

		if err := writeEnvAndRestart(client, appName, env); err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"status":  "removed",
				"app":     appName,
				"server":  srv.Host,
				"removed": len(args),
			})
		}
		fmt.Fprintf(os.Stderr, "Removed %d env var(s) and restarted %q on %s.\n", len(args), appName, srv.Host)
		return nil
	},
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environment variable names",
	Long: `List all environment variable names (without values by default).

Examples:
  tillandsia env list
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		appName, srv, client, err := connectForEnv(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		env, err := deploy.ReadEnvFile(context.Background(), client, appName)
		if err != nil {
			return fmt.Errorf("reading env file: %w", err)
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(envMapToSortedSlice(env))
		}

		if len(env) == 0 {
			fmt.Println("No environment variables set.")
			return nil
		}

		fmt.Printf("Environment variables for %q on %s:\n", appName, srv.Host)
		for _, k := range sortedKeys(env) {
			fmt.Printf("  %s\n", k)
		}
		return nil
	},
}

var envExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all environment variables",
	Long: `Export all environment variables as KEY=VALUE lines or JSON.

Examples:
  tillandsia env export
  tillandsia env export --json
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName, _ := cmd.Flags().GetString("server")
		appName, _, client, err := connectForEnv(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		env, err := deploy.ReadEnvFile(context.Background(), client, appName)
		if err != nil {
			return fmt.Errorf("reading env file: %w", err)
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(env)
		}

		for _, k := range sortedKeys(env) {
			fmt.Printf("%s=%s\n", k, env[k])
		}
		return nil
	},
}

func connectForEnv(serverName string) (string, *configResolvedServer, *ssh.Client, error) {
	cfgDir, err := config.FindConfigDir()
	if err != nil {
		return "", nil, nil, err
	}
	cfg, err := config.ReadConfig(cfgDir)
	if err != nil {
		return "", nil, nil, err
	}
	srv, err := resolveServer(serverName)
	if err != nil {
		return "", nil, nil, err
	}
	client, err := ssh.Connect(srv.Host, srv.User, srv.Key, srv.Port)
	if err != nil {
		return "", nil, nil, fmt.Errorf("connecting to server: %w", err)
	}
	return cfg.Name, srv, client, nil
}

func writeEnvAndRestart(client *ssh.Client, appName string, env map[string]string) error {
	if err := deploy.WriteEnvFile(context.Background(), client, appName, env); err != nil {
		return fmt.Errorf("writing env file: %w", err)
	}
	if err := deploy.RestartContainer(context.Background(), client, appName); err != nil {
		return fmt.Errorf("restarting container: %w", err)
	}
	return nil
}