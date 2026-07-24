package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "tillandsia",
	Short: "Deploy anywhere. No roots required.",
	Long: `Tillandsia is an open source, self-hostable deployment platform.
Deploy your application to any VPS with a single command.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format for agents")
	rootCmd.PersistentFlags().Bool("yes", false, "non-interactive mode, accept defaults")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug output")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
