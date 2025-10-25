package cli

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"skyport-agent/internal/auth"
	"skyport-agent/internal/config"
	"skyport-agent/internal/logger"
	"skyport-agent/internal/service"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage tunnels",
	Long:  `Manage your SkyPort tunnels - list, run, and monitor tunnel connections.`,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tunnels",
	Long: `List all tunnels associated with your account.

Example:
  skyport tunnel list`,
	Run: runList,
}

var runCmd = &cobra.Command{
	Use:   "run [tunnel-name-or-id]",
	Short: "Start a tunnel",
	Long: `Start a tunnel by name or ID. The tunnel will run until stopped with Ctrl+C.

Examples:
  skyport tunnel run myapp
  skyport tunnel run df35dc8d-fb0b-4abd-a75e-9609d83b3439`,
	Args: cobra.ExactArgs(1),
	Run:  runTunnel,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show tunnel status",
	Long: `Show the status of all active tunnel connections.

Example:
  skyport tunnel status`,
	Run: runStatus,
}

// Note: Worker command removed - tunnels now run directly in foreground

var stopCmd = &cobra.Command{
	Use:   "stop [tunnel-name-or-id]",
	Short: "Stop a running tunnel",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		nameOrID := args[0]

		// Resolve tunnel ID from server list
		defaultConfig := config.Load()
		authManager := auth.NewAuthManager(defaultConfig)
		if !authManager.IsAuthenticated() {
			fmt.Println(" You are not logged in. Please run 'skyport login' first.")
			os.Exit(1)
		}
		token, err := authManager.GetValidToken()
		if err != nil {
			fmt.Println(" Your session has expired. Please run 'skyport login' again.")
			os.Exit(1)
		}
		tunnels, err := authManager.FetchTunnels(token)
		if err != nil {
			log.Fatalf(" Failed to get tunnel list: %v", err)
		}
		var tunnelID string
		var tunnelName string
		for _, t := range tunnels {
			if t.ID == nameOrID || t.Name == nameOrID {
				tunnelID = t.ID
				tunnelName = t.Name
				break
			}
		}
		if tunnelID == "" {
			fmt.Printf(" Tunnel '%s' not found.\n", nameOrID)
			os.Exit(1)
		}

		// First, kill any local background daemon processes for this tunnel
		killBackgroundProcess(tunnelID, tunnelName)

		// Then send stop request to server API
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/tunnels/%s/stop", defaultConfig.ServerURL, tunnelID), nil)
		if err != nil {
			log.Fatalf(" Failed to create stop request: %v", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf(" Failed to stop tunnel: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Printf(" ✓ Stopped tunnel '%s'\n", nameOrID)
		} else if resp.StatusCode == http.StatusBadRequest {
			fmt.Println(" ⚠ Tunnel is not currently active")
		} else {
			fmt.Printf(" ✗ Failed to stop tunnel (status: %d)\n", resp.StatusCode)
		}
	},
}

func init() {
	tunnelCmd.AddCommand(listCmd)
	tunnelCmd.AddCommand(runCmd)
	tunnelCmd.AddCommand(statusCmd)
	tunnelCmd.AddCommand(stopCmd)

	// Flags for "run"
	runCmd.Flags().Bool("background", false, "Run tunnel in background")
	// runCmd.Flags().Bool("auto-start", false, "Mark tunnel to auto-start on boot (requires service)")

	// autostart subcommand
	autostartCmd := &cobra.Command{
		Use:    "autostart [tunnel-name-or-id] [enable|disable]",
		Short:  "Enable or disable auto-start for a tunnel",
		Args:   cobra.ExactArgs(2),
		Hidden: true, // Hide from help
		Run: func(cmd *cobra.Command, args []string) {
			nameOrID := args[0]
			action := args[1]

			defaultConfig := config.Load()
			manager := service.NewManager(defaultConfig)

			// Must be authenticated to resolve tunnel and persist
			if !manager.IsAuthenticated() {
				fmt.Println(" You are not logged in. Please run 'skyport login' first.")
				os.Exit(1)
			}

			// Sync tunnels so we have local IDs mapping
			if err := manager.SyncTunnelsFromServer(); err != nil {
				log.Printf(" Warning: Failed to sync tunnels from server: %v", err)
			}

			// Find tunnel ID by name or ID in local config
			tunnels, err := manager.GetTunnelList()
			if err != nil {
				log.Fatalf(" Failed to load tunnels: %v", err)
			}

			var tunnelID string
			for _, t := range tunnels {
				if t.ID == nameOrID || t.Name == nameOrID {
					tunnelID = t.ID
					break
				}
			}
			if tunnelID == "" {
				fmt.Printf(" Tunnel '%s' not found.\n", nameOrID)
				os.Exit(1)
			}

			enable := false
			switch action {
			case "enable":
				enable = true
			case "disable":
				enable = false
			default:
				fmt.Println(" Action must be 'enable' or 'disable'")
				os.Exit(1)
			}

			if err := manager.SetTunnelAutoStart(tunnelID, enable); err != nil {
				log.Fatalf(" Failed to update auto-start: %v", err)
			}

			state := "disabled"
			if enable {
				state = "enabled"
			}
			fmt.Printf(" Auto-start %s for tunnel '%s'\n", state, nameOrID)

			if enable {
				fmt.Println(" Note: To start on boot, install and start the service:")
				fmt.Println("   skyport service install && skyport service start")
			}
		},
	}
	tunnelCmd.AddCommand(autostartCmd)
}

func runList(cmd *cobra.Command, args []string) {
	if verbose {
		fmt.Println(" Loading tunnel list...")
	}

	// Create default config for auth manager
	defaultConfig := config.Load()
	authManager := auth.NewAuthManager(defaultConfig)

	// Check if user is authenticated using unified auth system
	if !authManager.IsAuthenticated() {
		fmt.Println(" You are not logged in. Please run 'skyport login' first.")
		os.Exit(1)
	}

	// Get user data from unified auth system
	userData, err := authManager.LoadCredentials()
	if err != nil {
		fmt.Println(" Your session has expired. Please run 'skyport login' again.")
		os.Exit(1)
	}

	if verbose {
		fmt.Printf(" Authenticated as %s\n", userData.Name)
	}

	// Prefer server as source of truth for status
	token, err := authManager.GetValidToken()
	if err != nil {
		fmt.Println(" Your session has expired. Please run 'skyport login' again.")
		os.Exit(1)
	}

	tunnelsFromServer, err := authManager.FetchTunnels(token)
	if err != nil {
		log.Fatalf(" Failed to get tunnel list: %v", err)
	}

	if len(tunnelsFromServer) == 0 {
		fmt.Println(" No tunnels found.")
		fmt.Printf("   Create tunnels at: %s/dashboard\n", defaultConfig.WebURL)
		return
	}

	fmt.Printf(" Found %d tunnel(s):\n\n", len(tunnelsFromServer))

	// Create a table writer for nice formatting
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSUBDOMAIN\tLOCAL PORT\tSTATUS")
	fmt.Fprintln(w, "----\t---------\t----------\t------")

	for _, tunnel := range tunnelsFromServer {
		status := " Stopped"
		if tunnel.IsActive {
			status = " Running"
		}

		// autoStart := "No"
		// if tunnel.AutoStart {
		// 	autoStart = "Yes"
		// }

		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
			tunnel.Name,
			tunnel.Subdomain,
			tunnel.LocalPort,
			status)
	}

	w.Flush()
	fmt.Println()
	fmt.Println(" Use 'skyport tunnel run <name>' to start a tunnel")
	fmt.Printf(" Access running tunnels at: http://<subdomain>.%s\n", defaultConfig.TunnelDomain)
}

func runTunnel(cmd *cobra.Command, args []string) {
	tunnelNameOrID := args[0]

	fmt.Printf(" Starting tunnel: %s\n", tunnelNameOrID)

	// Create default config for services
	defaultConfig := config.Load()
	authManager := auth.NewAuthManager(defaultConfig)

	// Check if user is authenticated using unified auth system
	if !authManager.IsAuthenticated() {
		fmt.Println(" ✗ You are not logged in. Please run 'skyport login' first.")
		os.Exit(1)
	}

	// Get token for server communication
	token, err := authManager.GetValidToken()
	if err != nil {
		fmt.Println(" ✗ Your session has expired. Please run 'skyport login' again.")
		os.Exit(1)
	}

	// Get tunnels from server to find target tunnel
	tunnelsFromServer, err := authManager.FetchTunnels(token)
	if err != nil {
		if config.IsDebugMode() {
			log.Fatalf(" Failed to get tunnel list: %v", err)
		} else {
			fmt.Println(" ✗ Failed to connect to SkyPort server")
			fmt.Println(" Please check your internet connection and try again")
			os.Exit(1)
		}
	}

	var targetTunnel *config.Tunnel
	for _, tunnel := range tunnelsFromServer {
		if tunnel.Name == tunnelNameOrID || tunnel.ID == tunnelNameOrID {
			targetTunnel = &tunnel
			break
		}
	}

	if targetTunnel == nil {
		fmt.Printf(" ✗ Tunnel '%s' not found.\n", tunnelNameOrID)
		fmt.Println(" Use 'skyport tunnel list' to see available tunnels")
		os.Exit(1)
	}

	// Check if tunnel is already running on server
	if targetTunnel.IsActive {
		fmt.Printf(" ⚠ Tunnel '%s' is already running\n", targetTunnel.Name)
		fmt.Println(" Use 'skyport tunnel stop", targetTunnel.Name, "' to stop it first")
		os.Exit(1)
	}

	// Start tunnel
	fmt.Printf(" Connecting %s (%s.%s → localhost:%d)\n",
		targetTunnel.Name,
		targetTunnel.Subdomain,
		defaultConfig.TunnelDomain,
		targetTunnel.LocalPort)

	// Create service manager and sync tunnels from server first
	manager := service.NewManager(defaultConfig)

	// Sync tunnels from server to local config before connecting
	if err := manager.SyncTunnelsFromServer(); err != nil {
		log.Printf(" Warning: Failed to sync tunnels from server: %v", err)
		// Continue anyway - the tunnel data is already available from FetchTunnels
	}

	// Check flags
	runInBackground, _ := cmd.Flags().GetBool("background")
	// setAutoStart, _ := cmd.Flags().GetBool("auto-start")

	if runInBackground {
		// Start a detached background process that connects this tunnel now
		exe, err := os.Executable()
		if err != nil {
			if config.IsDebugMode() {
				log.Fatalf(" Failed to resolve executable path: %v", err)
			} else {
				fmt.Println(" ✗ Failed to start tunnel")
				fmt.Println(" Please contact SkyPort support if this issue persists")
				os.Exit(1)
			}
		}

		// Create log file for background process (always create for debugging if needed)
		logDir := os.TempDir()
		logFile := fmt.Sprintf("%s/skyport-tunnel-%s.log", logDir, targetTunnel.Name)
		logFd, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			if config.IsDebugMode() {
				log.Fatalf(" Failed to create log file: %v", err)
			} else {
				fmt.Println(" ✗ Failed to start tunnel")
				fmt.Println(" Please contact SkyPort support if this issue persists")
				os.Exit(1)
			}
		}

		cmd := exec.Command(exe, "daemon", "--connect-tunnel", targetTunnel.ID, "--foreground")
		cmd.Stdout = logFd
		cmd.Stderr = logFd
		cmd.Stdin = nil
		configureDaemonProcess(cmd)

		if err := cmd.Start(); err != nil {
			logFd.Close()
			if config.IsDebugMode() {
				log.Fatalf(" Failed to start background process: %v", err)
			} else {
				fmt.Println(" ✗ Failed to start tunnel")
				fmt.Println(" Please contact SkyPort support if this issue persists")
				os.Exit(1)
			}
		}

		// Close the file descriptor in parent process (child process keeps it open)
		logFd.Close()

		// Show clean output to users
		fmt.Printf(" ✓ Started background process (pid %d) for tunnel '%s'\n", cmd.Process.Pid, targetTunnel.Name)

		// Only show log file location in debug mode
		if config.IsDebugMode() {
			fmt.Printf(" [DEBUG] Logs: %s\n", logFile)
			fmt.Printf(" [DEBUG] To view logs: tail -f %s\n", logFile)
		}

		fmt.Println(" To view status: skyport tunnel status")
		return
	}

	if err := manager.ConnectTunnel(targetTunnel.ID, false); err != nil {
		if config.IsDebugMode() {
			log.Fatalf(" Failed to start tunnel: %v", err)
		} else {
			fmt.Println(" ✗ Failed to start tunnel")
			fmt.Println(" Please check that your local service is running and try again")
			fmt.Println(" If the issue persists, contact SkyPort support")
			os.Exit(1)
		}
	}

	fmt.Printf(" ✓ Tunnel '%s' started successfully\n", targetTunnel.Name)
	fmt.Printf(" ✓ Access your service at: http://%s.%s\n", targetTunnel.Subdomain, defaultConfig.TunnelDomain)
	fmt.Println(" Press Ctrl+C to stop the tunnel")

	// Keep the tunnel running until interrupted
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\n Stopping tunnel...")

	// Disconnect the tunnel
	if err := manager.DisconnectTunnel(targetTunnel.ID); err != nil {
		if config.IsDebugMode() {
			log.Printf(" Warning: Failed to disconnect tunnel: %v", err)
		}
	}

	fmt.Println(" ✓ Tunnel stopped.")
}

func runStatus(cmd *cobra.Command, args []string) {
	if verbose {
		fmt.Println(" Checking tunnel status...")
	}

	// Create default config for services
	defaultConfig := config.Load()
	authManager := auth.NewAuthManager(defaultConfig)

	// Check if user is authenticated using unified auth system
	if !authManager.IsAuthenticated() {
		fmt.Println(" You are not logged in. Please run 'skyport login' first.")
		os.Exit(1)
	}

	// Prefer server as source of truth for status
	token, err := authManager.GetValidToken()
	if err != nil {
		fmt.Println(" Your session has expired. Please run 'skyport login' again.")
		os.Exit(1)
	}

	tunnelsFromServer, err := authManager.FetchTunnels(token)
	if err != nil {
		log.Fatalf(" Failed to get tunnel list: %v", err)
	}

	// Filter for active tunnels (server state)
	var activeTunnels []config.Tunnel
	for _, tunnel := range tunnelsFromServer {
		if tunnel.IsActive {
			activeTunnels = append(activeTunnels, tunnel)
		}
	}

	if len(activeTunnels) == 0 {
		fmt.Println(" No tunnels are currently running.")
		fmt.Println(" Use 'skyport tunnel run <name>' to start a tunnel")
		return
	}

	fmt.Printf(" Active tunnels (%d running):\n\n", len(activeTunnels))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSUBDOMAIN\tLOCAL PORT\tURL")
	fmt.Fprintln(w, "----\t---------\t----------\t---")

	for _, tunnel := range activeTunnels {
		url := fmt.Sprintf("http://%s.%s", tunnel.Subdomain, defaultConfig.TunnelDomain)
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
			tunnel.Name,
			tunnel.Subdomain,
			tunnel.LocalPort,
			url)
	}

	w.Flush()
	fmt.Println()
	fmt.Println("  Use Ctrl+C in the terminal running the tunnel to stop it")
}

// killBackgroundProcess finds and kills any background daemon process for the given tunnel
func killBackgroundProcess(tunnelID string, tunnelName string) {
	// Use ps to find processes matching "skyport daemon --connect-tunnel <tunnelID>"
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		logger.Debug("Failed to list processes: %v", err)
		return
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// Look for our daemon process with the tunnel ID
		if strings.Contains(line, "skyport") && strings.Contains(line, "daemon") &&
			strings.Contains(line, "--connect-tunnel") && strings.Contains(line, tunnelID) {
			// Extract PID (second field in ps aux output)
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid := fields[1]

			logger.Debug("Found background process (pid %s) for tunnel '%s', stopping it...", pid, tunnelName)

			// Kill the process
			killCmd := exec.Command("kill", pid)
			if err := killCmd.Run(); err != nil {
				logger.Debug("Failed to stop process %s: %v", pid, err)
			} else {
				logger.Info("Stopped background process for tunnel '%s'", tunnelName)
				// Give it a moment to terminate
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

// Note: PID file tracking removed - all tunnel state is now managed by the server
