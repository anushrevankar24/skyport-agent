package cli

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"skyport-agent/internal/config"
	"skyport-agent/internal/service"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run SkyPort agent as a daemon",
	Long: `Run the SkyPort agent as a background daemon with production-grade features:
- Automatic tunnel reconnection
- Health monitoring
- Network change detection
- Graceful shutdown handling
- System service integration`,
	Run: runDaemon,
}

var (
	daemonConfig = struct {
		configFile     string
		logLevel       string
		foreground     bool
		connectTunnels []string
	}{}
)

func init() {
	daemonCmd.Flags().StringVar(&daemonConfig.configFile, "config", "", "Path to configuration file")
	daemonCmd.Flags().StringVar(&daemonConfig.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	daemonCmd.Flags().BoolVar(&daemonConfig.foreground, "foreground", false, "Run in foreground (for debugging)")
	daemonCmd.Flags().StringSliceVar(&daemonConfig.connectTunnels, "connect-tunnel", []string{}, "Tunnel ID(s) to connect on start")
}

func runDaemon(cmd *cobra.Command, args []string) {
	fmt.Println("Starting SkyPort Agent Daemon...")

	// Load configuration
	cfg, err := loadDaemonConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create service manager
	manager := service.NewManager(cfg)

	// Create health monitor
	healthMonitor := service.NewHealthMonitor(manager)

	// Create network monitor
	networkMonitor := service.NewNetworkMonitor()

	// Start background manager
	manager.StartSilently()

	// If specific tunnels were requested, connect them explicitly (non-persistent)
	if len(daemonConfig.connectTunnels) > 0 {
		go func() {
			// Small delay to allow auth/monitors to initialize
			time.Sleep(500 * time.Millisecond)
			for _, tID := range daemonConfig.connectTunnels {
				if err := manager.ConnectTunnel(tID, false); err != nil {
					log.Printf("Failed to connect requested tunnel %s: %v", tID, err)
				} else {
					log.Printf("Connected requested tunnel: %s", tID)
				}
			}
		}()
	}

	// Start health monitoring
	healthMonitor.Start()

	// Start network monitoring
	networkMonitor.Start()

	// Handle network changes
	go handleNetworkChanges(networkMonitor, manager)

	// Setup signal handling
	setupSignalHandling(manager, healthMonitor, networkMonitor)

	// Log startup
	log.Printf("SkyPort Agent Daemon started successfully")
	log.Printf("Configuration: %+v", daemonConfig)

	// Keep running
	if daemonConfig.foreground {
		// Run in foreground for debugging
		runForeground(manager, healthMonitor, networkMonitor)
	} else {
		// Run as daemon
		runBackground(manager, healthMonitor, networkMonitor)
	}
}

func loadDaemonConfig() (*config.Config, error) {
	return config.Load(), nil
}

func handleNetworkChanges(networkMonitor *service.NetworkMonitor, manager *service.Manager) {
	changeChan := networkMonitor.GetChangeChannel()

	for change := range changeChan {
		log.Printf("Network change detected: %s", change.Description)

		// Handle different types of network changes
		switch change.Type {
		case "ip_change":
			handleIPChange(manager)
		case "interface_change":
			handleInterfaceChange(manager)
		default:
			log.Printf("Unknown network change type: %s", change.Type)
		}
	}
}

func handleIPChange(manager *service.Manager) {
	log.Println("Handling IP address change...")

	// Disconnect all tunnels
	activeTunnels := manager.GetActiveTunnels()
	for _, tunnelID := range activeTunnels {
		log.Printf("Disconnecting tunnel %s due to IP change", tunnelID)
		if err := manager.DisconnectTunnel(tunnelID); err != nil {
			log.Printf("Error disconnecting tunnel %s: %v", tunnelID, err)
		}
	}

	// Wait a moment for disconnections to complete
	time.Sleep(2 * time.Second)

	// Reconnect tunnels
	for _, tunnelID := range activeTunnels {
		log.Printf("Reconnecting tunnel %s after IP change", tunnelID)
		if err := manager.ConnectTunnel(tunnelID, false); err != nil {
			log.Printf("Error reconnecting tunnel %s: %v", tunnelID, err)
		}
	}
}

func handleInterfaceChange(manager *service.Manager) {
	log.Println("Handling network interface change...")

	// Similar to IP change, but may need different handling
	// For now, treat it the same as IP change
	handleIPChange(manager)
}

func setupSignalHandling(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("Received signal %v, shutting down gracefully", sig)
				gracefulShutdown(manager, healthMonitor, networkMonitor)
				os.Exit(0)
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, reloading configuration")
				// TODO: Implement configuration reload
			}
		}
	}()
}

func gracefulShutdown(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	log.Println("Starting graceful shutdown...")

	// Stop network monitoring
	networkMonitor.Stop()

	// Stop health monitoring
	healthMonitor.Stop()

	// Stop manager
	manager.StopSilently()

	log.Println("Graceful shutdown complete")
}

func runForeground(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	fmt.Println("Running in foreground mode...")
	fmt.Println("Press Ctrl+C to stop")

	// Keep running until interrupted
	select {}
}

func runBackground(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	// Run as background daemon
	for {
		time.Sleep(1 * time.Minute)

		// Periodic status check
		status := healthMonitor.GetHealthStatus()
		log.Printf("Daemon status: %+v", status)
	}
}
