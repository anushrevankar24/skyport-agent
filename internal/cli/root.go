package cli

import (
	"fmt"
	"os"
	"skyport-agent/internal/config"
	"skyport-agent/internal/network"

	"github.com/spf13/cobra"
)

var (
	version = "1.0.0"
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "skyport",
	Short: "SkyPort - Secure tunnel client",
	Long: `SkyPort is a secure tunnel client that allows you to expose local services 
to the internet through encrypted tunnels.

Features:
- Secure authentication via browser login
- Easy tunnel management
- Automatic background connections
- HTTP/HTTPS/WebSocket support`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip network check for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "skyport" || cmd.Name() == "uninstall" {
			return nil
		}

		// Check network connectivity before running any command
		cfg := config.Load()
		if err := network.CheckConnectivity(cfg); err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("\nPlease ensure:")
			fmt.Println("  - You have an active internet connection")
			fmt.Println("  - The SkyPort server is running")
			os.Exit(1)
		}

		if verbose {
			fmt.Println("Network connectivity verified")
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add subcommands
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(tunnelCmd)
	// rootCmd.AddCommand(daemonCmd) // Hidden for now
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(agentStatusCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(uninstallAgentCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("SkyPort CLI v%s\n", version)
	},
}
