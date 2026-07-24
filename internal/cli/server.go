package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/tillandsia/tillandsia/internal/config"
	"github.com/tillandsia/tillandsia/internal/server"
	"github.com/tillandsia/tillandsia/internal/types"
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"srv", "host"},
	Short:   "Manage deployment servers",
	Long:    `Add, list, inspect, setup, and remove deployment servers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var serverAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register a new server",
	Long: `Register a server for deployment.

The server must be reachable via SSH. Provide the hostname or IP, SSH user,
and optionally an SSH key path and port.

Examples:
  tillandsia server add my-vps --host 203.0.113.42
  tillandsia server add my-vps --host 203.0.113.42 --user root --port 22
  tillandsia server add my-vps --host 203.0.113.42 --key ~/.ssh/id_rsa
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sc, err := config.ReadServersConfig()
		if err != nil {
			return err
		}
		if _, exists := sc.Servers[name]; exists {
			return fmt.Errorf("server %q already exists", name)
		}

		host, _ := cmd.Flags().GetString("host")
		if host == "" {
			return fmt.Errorf("--host is required")
		}
		user, _ := cmd.Flags().GetString("user")
		keyPath, _ := cmd.Flags().GetString("key")
		port, _ := cmd.Flags().GetInt("port")
		setDefault, _ := cmd.Flags().GetBool("default")

		if port == 0 {
			port = 22
		}
		if user == "" {
			user = "root"
		}

		// Validate SSH connectivity before adding
		fmt.Fprintf(os.Stderr, "Validating SSH connectivity to %s@%s:%d... ", user, host, port)
		if err := server.Test(context.Background(), host, user, keyPath, port); err != nil {
			fmt.Fprintln(os.Stderr, "FAILED")
			return fmt.Errorf("SSH validation failed: %w", err)
		}
		fmt.Fprintln(os.Stderr, "OK")

		if setDefault {
			for _, s := range sc.Servers {
				s.Default = false
			}
		}

		sc.Servers[name] = &types.Server{
			Host:    host,
			User:    user,
			Key:     keyPath,
			Port:    port,
			Default: setDefault,
		}

		if err := config.WriteServersConfig(sc); err != nil {
			return err
		}

		if jsonOutput {
			out := map[string]interface{}{
				"name":   name,
				"host":   host,
				"user":   user,
				"port":   port,
				"status": "added",
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		fmt.Printf("Server %q added successfully.\n", name)
		return nil
	},
}

var serverSetupCmd = &cobra.Command{
	Use:   "setup <name>",
	Short: "Install Docker and prepare a server",
	Long: `Install Docker, create data directories, and verify the server is ready for deployment.

Connects via SSH and:
  - Installs Docker if not already present
  - Creates /var/lib/tillandsia/ for app data
  - Verifies Docker is running with a test container

Examples:
  tillandsia server setup my-vps
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sc, err := config.ReadServersConfig()
		if err != nil {
			return err
		}
		srv, ok := sc.Servers[name]
		if !ok {
			return fmt.Errorf("server %q not found. Use 'tillandsia server add' first", name)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Fprintf(os.Stderr, "This will install Docker on %s (%s@%s). Continue? [y/N] ", name, srv.User, srv.Host)
			var resp string
			fmt.Scanln(&resp)
			if resp != "y" && resp != "Y" {
				return fmt.Errorf("aborted")
			}
		}

		fmt.Fprintf(os.Stderr, "Setting up server %s (%s@%s)...\n", name, srv.User, srv.Host)

		if err := server.Setup(context.Background(), srv.Host, srv.User, srv.Key, srv.Port); err != nil {
			return err
		}

		if jsonOutput {
			out := map[string]interface{}{
				"name":   name,
				"host":   srv.Host,
				"status": "ready",
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		fmt.Printf("Server %q is ready for deployment.\n", name)
		return nil
	},
}

var serverLsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List registered servers",
	Long:    `List all registered servers and their connection details.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sc, err := config.ReadServersConfig()
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(sc.Servers)
		}

		if len(sc.Servers) == 0 {
			fmt.Println("No servers registered.")
			return nil
		}

		var names []string
		for n := range sc.Servers {
			names = append(names, n)
		}
		sort.Strings(names)

		fmt.Println("Servers:")
		for _, n := range names {
			s := sc.Servers[n]
			def := ""
			if s.Default {
				def = " (default)"
			}
			fmt.Printf("  %s  %s@%s:%d%s\n", n, s.User, s.Host, s.Port, def)
		}
		return nil
	},
}

var serverInspectCmd = &cobra.Command{
	Use:   "inspect <name>",
	Short: "Show server details and connectivity status",
	Long:  `Show detailed information about a registered server and test SSH connectivity.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sc, err := config.ReadServersConfig()
		if err != nil {
			return err
		}
		srv, ok := sc.Servers[name]
		if !ok {
			return fmt.Errorf("server %q not found", name)
		}

		connErr := server.Test(context.Background(), srv.Host, srv.User, srv.Key, srv.Port)
		reachable := connErr == nil

		if jsonOutput {
			out := map[string]interface{}{
				"name":      name,
				"host":      srv.Host,
				"user":      srv.User,
				"port":      srv.Port,
				"key":       srv.Key,
				"default":   srv.Default,
				"reachable": reachable,
			}
			if !reachable {
				out["error"] = connErr.Error()
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		status := "unreachable"
		if reachable {
			status = "reachable"
		}
		fmt.Printf("Name:      %s\n", name)
		fmt.Printf("Host:      %s\n", srv.Host)
		fmt.Printf("User:      %s\n", srv.User)
		fmt.Printf("Port:      %d\n", srv.Port)
		fmt.Printf("Key:       %s\n", srv.Key)
		fmt.Printf("Default:   %t\n", srv.Default)
		fmt.Printf("Status:    %s\n", status)
		if !reachable {
			fmt.Fprintf(os.Stderr, "Error:     %s\n", connErr)
		}
		return nil
	},
}

var serverRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a registered server",
	Long:  `Remove a server from the configuration. Does not modify the server itself.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sc, err := config.ReadServersConfig()
		if err != nil {
			return err
		}
		if _, ok := sc.Servers[name]; !ok {
			return fmt.Errorf("server %q not found", name)
		}

		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			fmt.Fprintf(os.Stderr, "Remove server %q from configuration? [y/N] ", name)
			var resp string
			fmt.Scanln(&resp)
			if resp != "y" && resp != "Y" {
				return fmt.Errorf("aborted")
			}
		}

		delete(sc.Servers, name)
		if err := config.WriteServersConfig(sc); err != nil {
			return err
		}

		if jsonOutput {
			out := map[string]interface{}{
				"name":   name,
				"status": "removed",
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		fmt.Printf("Server %q removed.\n", name)
		return nil
	},
}

var serverDefaultCmd = &cobra.Command{
	Use:   "default <name>",
	Short: "Set the default server for deployment",
	Long:  `Set a registered server as the default target for deploy commands.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		sc, err := config.ReadServersConfig()
		if err != nil {
			return err
		}
		if _, ok := sc.Servers[name]; !ok {
			return fmt.Errorf("server %q not found", name)
		}

		for _, s := range sc.Servers {
			s.Default = false
		}
		sc.Servers[name].Default = true

		if err := config.WriteServersConfig(sc); err != nil {
			return err
		}

		if jsonOutput {
			out := map[string]interface{}{
				"name":   name,
				"status": "set as default",
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		}

		fmt.Printf("Server %q set as default.\n", name)
		return nil
	},
}

var serverAddFlags struct {
	host    string
	user    string
	key     string
	port    int
	setDefault bool
}

func init() {
	serverAddCmd.Flags().StringVar(&serverAddFlags.host, "host", "", "Server hostname or IP address")
	serverAddCmd.Flags().StringVar(&serverAddFlags.user, "user", "", "SSH user (default: root)")
	serverAddCmd.Flags().StringVar(&serverAddFlags.key, "key", "", "Path to SSH private key")
	serverAddCmd.Flags().IntVar(&serverAddFlags.port, "port", 22, "SSH port")
	serverAddCmd.Flags().BoolVar(&serverAddFlags.setDefault, "default", false, "Set as the default server")

	serverCmd.AddCommand(serverAddCmd)
	serverCmd.AddCommand(serverSetupCmd)
	serverCmd.AddCommand(serverLsCmd)
	serverCmd.AddCommand(serverInspectCmd)
	serverCmd.AddCommand(serverRmCmd)
	serverCmd.AddCommand(serverDefaultCmd)

	rootCmd.AddCommand(serverCmd)
}