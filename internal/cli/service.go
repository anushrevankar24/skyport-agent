package cli

import (
	"fmt"
	"log"
	"skyport-agent/internal/service"
	"strings"

	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage SkyPort agent as a system service",
	Long: `Manage the SkyPort agent as a systemd service with commands:
- install: Install the agent as a system service
- uninstall: Remove the agent service
- start: Start the agent service
- stop: Stop the agent service
- restart: Restart the agent service
- status: Show service status
- logs: Show service logs`,
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install SkyPort agent as a system service",
	Long: `Install the SkyPort agent as a systemd service that will:
- Start automatically on system boot
- Restart automatically if it crashes
- Run in the background with full persistence`,
	Run: runInstall,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove SkyPort agent system service",
	Long:  `Remove the SkyPort agent systemd service and stop it from running automatically.`,
	Run:   runUninstall,
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the SkyPort agent service",
	Long:  `Start the SkyPort agent systemd service.`,
	Run:   runStart,
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SkyPort agent service",
	Long:  `Stop the SkyPort agent systemd service.`,
	Run:   runServiceStop,
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the SkyPort agent service",
	Long:  `Restart the SkyPort agent systemd service.`,
	Run:   runServiceRestart,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SkyPort agent service status",
	Long:  `Show the current status of the SkyPort agent systemd service.`,
	Run:   runServiceStatus,
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show SkyPort agent service logs",
	Long:  `Show recent logs from the SkyPort agent systemd service.`,
	Run:   runLogs,
}

func init() {
	serviceCmd.AddCommand(installCmd)
	serviceCmd.AddCommand(uninstallCmd)
	serviceCmd.AddCommand(startCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(logsCmd)
}

func runInstall(cmd *cobra.Command, args []string) {
	fmt.Println("Installing SkyPort agent as system service...")

	systemdService := service.NewSystemdService()

	// Check if already installed
	if systemdService.IsInstalled() {
		fmt.Println("Service is already installed")
		return
	}

	// Install the service
	if err := systemdService.Install(); err != nil {
		log.Fatalf("Failed to install service: %v", err)
	}

	fmt.Println("Service installed successfully!")
	fmt.Println("Use 'skyport service start' to start the service")
	fmt.Println("Use 'skyport service status' to check service status")
}

func runUninstall(cmd *cobra.Command, args []string) {
	fmt.Println("Uninstalling SkyPort agent system service...")

	systemdService := service.NewSystemdService()

	// Check if installed
	if !systemdService.IsInstalled() {
		fmt.Println("Service is not installed")
		return
	}

	// Uninstall the service
	if err := systemdService.Uninstall(); err != nil {
		log.Fatalf("Failed to uninstall service: %v", err)
	}

	fmt.Println("Service uninstalled successfully!")
}

func runStart(cmd *cobra.Command, args []string) {
	fmt.Println("Starting SkyPort agent service...")

	systemdService := service.NewSystemdService()

	// Check if installed
	if !systemdService.IsInstalled() {
		fmt.Println("Service is not installed. Run 'skyport service install' first")
		return
	}

	// Start the service
	if err := systemdService.Start(); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}

	fmt.Println("Service started successfully!")
}

func runServiceStop(cmd *cobra.Command, args []string) {
	fmt.Println("Stopping SkyPort agent service...")

	systemdService := service.NewSystemdService()

	// Check if installed
	if !systemdService.IsInstalled() {
		fmt.Println("Service is not installed")
		return
	}

	// Stop the service
	if err := systemdService.Stop(); err != nil {
		log.Fatalf("Failed to stop service: %v", err)
	}

	fmt.Println("Service stopped successfully!")
}

func runServiceRestart(cmd *cobra.Command, args []string) {
	fmt.Println("Restarting SkyPort agent service...")

	systemdService := service.NewSystemdService()

	// Check if installed
	if !systemdService.IsInstalled() {
		fmt.Println("Service is not installed. Run 'skyport service install' first")
		return
	}

	// Restart the service
	if err := systemdService.Restart(); err != nil {
		log.Fatalf("Failed to restart service: %v", err)
	}

	fmt.Println("Service restarted successfully!")
}

func runServiceStatus(cmd *cobra.Command, args []string) {
	systemdService := service.NewSystemdService()

	// Check if installed
	if !systemdService.IsInstalled() {
		fmt.Println("Service is not installed")
		return
	}

	// Get service status
	status, err := systemdService.Status()
	if err != nil {
		log.Fatalf("Failed to get service status: %v", err)
	}

	// Get network info
	networkMonitor := service.NewNetworkMonitor()
	networkInfo := networkMonitor.GetCurrentNetworkInfo()

	// Display status
	fmt.Println("SkyPort Agent Service Status:")
	fmt.Printf("  Status: %s\n", status)
	fmt.Printf("  Installed: %t\n", systemdService.IsInstalled())
	fmt.Printf("  Running: %t\n", systemdService.IsRunning())
	fmt.Printf("  Network IP: %s\n", networkInfo["current_ip"])
	fmt.Printf("  Interface: %s\n", networkInfo["current_interface"])

	// Show service management commands
	fmt.Println("\nService Management Commands:")
	fmt.Println("  skyport service start    - Start the service")
	fmt.Println("  skyport service stop     - Stop the service")
	fmt.Println("  skyport service restart  - Restart the service")
	fmt.Println("  skyport service logs     - Show service logs")
	fmt.Println("  skyport service uninstall - Remove the service")
}

func runLogs(cmd *cobra.Command, args []string) {
	systemdService := service.NewSystemdService()

	// Check if installed
	if !systemdService.IsInstalled() {
		fmt.Println("Service is not installed")
		return
	}

	// Get service logs
	logs, err := systemdService.GetLogs(50) // Last 50 lines
	if err != nil {
		log.Fatalf("Failed to get service logs: %v", err)
	}

	fmt.Println("SkyPort Agent Service Logs:")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println(logs)
}
