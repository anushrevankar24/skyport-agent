package cli

import (
	"fmt"
	"skyport-agent/internal/config"
	"skyport-agent/internal/service"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var agentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SkyPort agent status and health information",
	Long: `Show comprehensive status information about the SkyPort agent including:
- Service status
- Active tunnels
- Health monitoring
- Network information
- System service status`,
	Run: runAgentStatus,
}

func runAgentStatus(cmd *cobra.Command, args []string) {
	fmt.Println("SkyPort Agent Status")
	fmt.Println(strings.Repeat("=", 50))

	// Check if running as service
	systemdService := service.NewSystemdService()
	if systemdService.IsInstalled() {
		status, _ := systemdService.Status()
		fmt.Printf("Service Status: %s\n", status)
		fmt.Printf("Service Running: %t\n", systemdService.IsRunning())
	} else {
		fmt.Println("Service Status: Not installed")
	}

	// Create manager to get status
	defaultConfig := config.Load()
	manager := service.NewManager(defaultConfig)

	// Check authentication
	if manager.IsAuthenticated() {
		fmt.Println("Authentication: Authenticated")

		// Get tunnel list
		tunnels, err := manager.GetTunnelList()
		if err != nil {
			fmt.Printf("Tunnel List: Error - %v\n", err)
		} else {
			fmt.Printf("Tunnel List: %d tunnels configured\n", len(tunnels))

			// Show active tunnels
			activeTunnels := manager.GetActiveTunnels()
			fmt.Printf("Active Tunnels: %d running\n", len(activeTunnels))

			if len(activeTunnels) > 0 {
				fmt.Println("\nActive Tunnel Details:")
				for _, tunnelID := range activeTunnels {
					// Find tunnel details
					for _, tunnel := range tunnels {
						if tunnel.ID == tunnelID {
							fmt.Printf("  - %s (%s.%s → localhost:%d)\n",
								tunnel.Name, tunnel.Subdomain, defaultConfig.TunnelDomain, tunnel.LocalPort)
							break
						}
					}
				}
			}
		}
	} else {
		fmt.Println("Authentication: Not authenticated")
		fmt.Println("Run 'skyport login' to authenticate")
	}

	// Get health status
	healthStatus := manager.GetHealthStatus()
	if len(healthStatus) > 0 {
		fmt.Println("\nHealth Monitoring:")
		fmt.Printf("  Active Tunnels: %v\n", healthStatus["active_tunnels"])
		fmt.Printf("  Reconnect Queue: %v\n", healthStatus["reconnect_queue"])
		fmt.Printf("  Last Health Check: %v\n", healthStatus["last_health_check"])
	}

	// Get network information
	networkInfo := manager.GetNetworkInfo()
	if len(networkInfo) > 0 {
		fmt.Println("\nNetwork Information:")
		fmt.Printf("  Current IP: %v\n", networkInfo["current_ip"])
		fmt.Printf("  Interface: %v\n", networkInfo["current_interface"])
		fmt.Printf("  Monitoring: %v\n", networkInfo["monitoring"])
	}

	// Show service management commands
	fmt.Println("\nService Management Commands:")
	fmt.Println("  skyport service install   - Install as system service")
	fmt.Println("  skyport service start     - Start the service")
	fmt.Println("  skyport service stop      - Stop the service")
	fmt.Println("  skyport service restart   - Restart the service")
	fmt.Println("  skyport service status    - Show service status")
	fmt.Println("  skyport service logs      - Show service logs")
	fmt.Println("  skyport service uninstall - Remove the service")

	// Show daemon commands
	fmt.Println("\nDaemon Commands:")
	fmt.Println("  skyport daemon           - Run as daemon (foreground)")
	fmt.Println("  skyport daemon --foreground - Run in foreground mode")

	// Show tunnel commands
	fmt.Println("\nTunnel Commands:")
	fmt.Println("  skyport tunnel list      - List all tunnels")
	fmt.Println("  skyport tunnel run <name> - Start a tunnel")
	fmt.Println("  skyport tunnel stop <name> - Stop a tunnel")
	fmt.Println("  skyport tunnel status    - Show tunnel status")

	fmt.Printf("\nStatus generated at: %s\n", time.Now().Format(time.RFC3339))
}
