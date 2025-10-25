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
	"skyport-agent/internal/service"
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

		// Send stop request to server API
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
			fmt.Printf(" Stopped tunnel '%s'\n", nameOrID)
		} else if resp.StatusCode == http.StatusBadRequest {
			fmt.Println("  Tunnel is not currently active.")
		} else {
			fmt.Printf(" Failed to stop tunnel (status: %d)\n", resp.StatusCode)
		}
	},
}

func init() {
	tunnelCmd.AddCommand(listCmd)
	tunnelCmd.AddCommand(runCmd)
	tunnelCmd.AddCommand(statusCmd)
	tunnelCmd.AddCommand(stopCmd)

	// Flags for "run"
	runCmd.Flags().Bool("background", false, "Run tunnel in background (daemon mode)")
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
		fmt.Println(" You are not logged in. Please run 'skyport login' first.")
		os.Exit(1)
	}

	// Get token for server communication
	token, err := authManager.GetValidToken()
	if err != nil {
		fmt.Println(" Your session has expired. Please run 'skyport login' again.")
		os.Exit(1)
	}

	// Get tunnels from server to find target tunnel
	tunnelsFromServer, err := authManager.FetchTunnels(token)
	if err != nil {
		log.Fatalf(" Failed to get tunnel list: %v", err)
	}

	var targetTunnel *config.Tunnel
	for _, tunnel := range tunnelsFromServer {
		if tunnel.Name == tunnelNameOrID || tunnel.ID == tunnelNameOrID {
			targetTunnel = &tunnel
			break
		}
	}

	if targetTunnel == nil {
		fmt.Printf(" Tunnel '%s' not found.\n", tunnelNameOrID)
		fmt.Println(" Use 'skyport tunnel list' to see available tunnels")
		os.Exit(1)
	}

	// Check if tunnel is already running on server
	if targetTunnel.IsActive {
		fmt.Printf("  Tunnel '%s' is already running on the server\n", targetTunnel.Name)
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
		// Optional: mark for auto-start persistence
		// if setAutoStart {
		// 	if err := manager.SetTunnelAutoStart(targetTunnel.ID, true); err != nil {
		// 		log.Fatalf(" Failed to set auto-start: %v", err)
		// 	}
		// 	fmt.Println(" Marked for auto-start on boot (requires service).")
		// }

		// Start a detached daemon that connects this tunnel now
		exe, err := os.Executable()
		if err != nil {
			log.Fatalf(" Failed to resolve executable path: %v", err)
		}
		cmd := exec.Command(exe, "daemon", "--connect-tunnel", targetTunnel.ID)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Stdin = nil
		configureDaemonProcess(cmd)

		if err := cmd.Start(); err != nil {
			log.Fatalf(" Failed to start background daemon: %v", err)
		}
		fmt.Printf(" Started background daemon (pid %d) for tunnel '%s'\n", cmd.Process.Pid, targetTunnel.Name)
		fmt.Println(" To view status: skyport service status (if installed) or 'ps' + logs")
		return
	}

	if err := manager.ConnectTunnel(targetTunnel.ID, false); err != nil {
		log.Fatalf(" Failed to start tunnel: %v", err)
	}

	fmt.Printf(" Tunnel '%s' started successfully\n", targetTunnel.Name)
	fmt.Printf(" Access your service at: http://%s.%s\n", targetTunnel.Subdomain, defaultConfig.TunnelDomain)
	fmt.Println("  To stop: skyport tunnel stop", targetTunnel.Name)
	fmt.Println("  Press Ctrl+C to stop the tunnel")

	// Keep the tunnel running until interrupted
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\n Stopping tunnel...")

	// Disconnect the tunnel
	if err := manager.DisconnectTunnel(targetTunnel.ID); err != nil {
		log.Printf(" Warning: Failed to disconnect tunnel: %v", err)
	}

	fmt.Println(" Tunnel stopped.")
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

// Note: PID file tracking removed - all tunnel state is now managed by the server
