package cli

import (
	"os"
	"os/signal"
	"skyport-agent/internal/config"
	"skyport-agent/internal/logger"
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
	logger.Debug("Starting SkyPort Agent Daemon...")

	// Load configuration
	cfg, err := loadDaemonConfig()
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	logger.Debug("Configuration loaded successfully")
	logger.Debug("Server URL: %s", cfg.ServerURL)
	logger.Debug("Tunnel Domain: %s", cfg.TunnelDomain)

	// Create service manager
	manager := service.NewManager(cfg)
	logger.Debug("Service manager created")

	// Create health monitor
	healthMonitor := service.NewHealthMonitor(manager)
	logger.Debug("Health monitor created")

	// Create network monitor
	networkMonitor := service.NewNetworkMonitor()
	logger.Debug("Network monitor created")

	// Start background manager
	manager.StartSilently()
	logger.Debug("Background manager started")

	// If specific tunnels were requested, connect them explicitly with auto-reconnect
	if len(daemonConfig.connectTunnels) > 0 {
		logger.Debug("Connecting %d requested tunnel(s)...", len(daemonConfig.connectTunnels))
		go func() {
			// Small delay to allow auth/monitors to initialize
			time.Sleep(500 * time.Millisecond)
			for _, tID := range daemonConfig.connectTunnels {
				logger.Debug("Attempting to connect tunnel: %s", tID)
				// Enable auto-reconnect (true) so tunnel stays connected
				if err := manager.ConnectTunnel(tID, true); err != nil {
					logger.Error("Failed to connect tunnel %s: %v", tID, err)
				} else {
					logger.Info("Connected tunnel: %s (auto-reconnect enabled)", tID)
				}
			}
		}()
	}

	// Start health monitoring
	healthMonitor.Start()
	logger.Debug("Health monitoring started")

	// Start network monitoring
	networkMonitor.Start()
	logger.Debug("Network monitoring started")

	// Handle network changes
	go handleNetworkChanges(networkMonitor, manager)

	// Setup signal handling
	setupSignalHandling(manager, healthMonitor, networkMonitor)

	// Log startup
	logger.Info("SkyPort Agent Daemon started successfully")
	logger.Debug("Daemon configuration: %+v", daemonConfig)

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
		logger.Info("Network change detected: %s", change.Description)

		// Handle different types of network changes
		switch change.Type {
		case "ip_change":
			handleIPChange(manager)
		case "interface_change":
			handleInterfaceChange(manager)
		default:
			logger.Debug("Unknown network change type: %s", change.Type)
		}
	}
}

func handleIPChange(manager *service.Manager) {
	logger.Debug("Handling IP address change...")

	// Disconnect all tunnels
	activeTunnels := manager.GetActiveTunnels()
	for _, tunnelID := range activeTunnels {
		logger.Debug("Disconnecting tunnel %s due to IP change", tunnelID)
		if err := manager.DisconnectTunnel(tunnelID); err != nil {
			logger.Error("Error disconnecting tunnel %s: %v", tunnelID, err)
		}
	}

	// Wait a moment for disconnections to complete
	time.Sleep(2 * time.Second)

	// Reconnect tunnels
	for _, tunnelID := range activeTunnels {
		logger.Info("Reconnecting tunnel %s after IP change", tunnelID)
		if err := manager.ConnectTunnel(tunnelID, false); err != nil {
			logger.Error("Error reconnecting tunnel %s: %v", tunnelID, err)
		}
	}
}

func handleInterfaceChange(manager *service.Manager) {
	logger.Debug("Handling network interface change...")

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
				logger.Info("Received signal %v, shutting down gracefully", sig)
				gracefulShutdown(manager, healthMonitor, networkMonitor)
				os.Exit(0)
			case syscall.SIGHUP:
				logger.Debug("Received SIGHUP, reloading configuration")
				// TODO: Implement configuration reload
			}
		}
	}()
}

func gracefulShutdown(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	logger.Debug("Starting graceful shutdown...")

	// Stop network monitoring
	networkMonitor.Stop()
	logger.Debug("Network monitoring stopped")

	// Stop health monitoring
	healthMonitor.Stop()
	logger.Debug("Health monitoring stopped")

	// Stop manager
	manager.StopSilently()
	logger.Debug("Manager stopped")

	logger.Info("Graceful shutdown complete")
}

func runForeground(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	logger.Info("Running in foreground mode...")
	logger.Info("Press Ctrl+C to stop")

	// Keep running until interrupted
	select {}
}

func runBackground(manager *service.Manager, healthMonitor *service.HealthMonitor, networkMonitor *service.NetworkMonitor) {
	// Run as background daemon
	for {
		time.Sleep(1 * time.Minute)

		// Periodic status check
		status := healthMonitor.GetHealthStatus()
		logger.Debug("Daemon status: %+v", status)
	}
}
