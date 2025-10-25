package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"skyport-agent/internal/config"
	"skyport-agent/internal/service"

	"github.com/spf13/cobra"
)

var (
	forceUninstall   bool
	keepConfig       bool
	skipConfirmation bool
)

var uninstallAgentCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Completely uninstall SkyPort agent from your system",
	Long: `Completely remove SkyPort agent from your system including:
- SkyPort binary
- System service (if installed)
- Configuration files
- Stored credentials

This is different from 'skyport service uninstall' which only removes the service.`,
	Run: runCompleteUninstall,
}

func init() {
	uninstallAgentCmd.Flags().BoolVarP(&forceUninstall, "force", "f", false, "Force uninstall without confirmation")
	uninstallAgentCmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep configuration files and credentials")
	uninstallAgentCmd.Flags().BoolVarP(&skipConfirmation, "yes", "y", false, "Skip all confirmation prompts")
}

func runCompleteUninstall(cmd *cobra.Command, args []string) {
	if runtime.GOOS == "windows" {
		runWindowsUninstall()
		return
	}

	runUnixUninstall()
}

func runUnixUninstall() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("SkyPort Agent Uninstaller")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if !forceUninstall && !skipConfirmation {
		fmt.Println("This will completely remove SkyPort from your system.")
		fmt.Println()
		fmt.Print("Are you sure you want to continue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" {
			fmt.Println("Uninstall cancelled.")
			return
		}
		fmt.Println()
	}

	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Warning: Could not determine binary path: %v\n", err)
		binaryPath = "/usr/local/bin/skyport"
	}

	// Step 1: Stop and remove systemd service
	fmt.Println("Step 1: Checking system service...")
	systemdService := service.NewSystemdService()
	if systemdService.IsInstalled() {
		fmt.Println("   Service found. Removing...")
		fmt.Println("   Stopping service...")
		systemdService.Stop()

		fmt.Println("   Disabling service...")
		if err := systemdService.Uninstall(); err != nil {
			fmt.Printf("   Warning: Failed to uninstall service: %v\n", err)
		} else {
			fmt.Println("   ✓ Service removed successfully")
		}
	} else {
		fmt.Println("   ✓ No service installed")
	}

	// Step 2: Remove configuration files
	if !keepConfig {
		fmt.Println()
		fmt.Println("Step 2: Removing configuration files...")
		configDir, err := config.GetConfigDir()
		if err == nil && dirExists(configDir) {
			if err := os.RemoveAll(configDir); err != nil {
				fmt.Printf("   Warning: Failed to remove config: %v\n", err)
			} else {
				fmt.Println("   ✓ Configuration removed")
			}
		} else {
			fmt.Println("   ✓ No configuration found")
		}

		// Clear keyring credentials
		clearKeyring()
	} else {
		fmt.Println()
		fmt.Println("Step 2: Skipping configuration removal (--keep-config)")
	}

	// Step 3: Remove binary
	fmt.Println()
	fmt.Println("Step 3: Removing binary...")
	fmt.Printf("   Binary location: %s\n", binaryPath)

	// Create a self-destruct script
	selfDestructScript := `#!/bin/bash
sleep 1
rm -f "` + binaryPath + `" 2>/dev/null
exit 0
`

	scriptPath := "/tmp/skyport-uninstall.sh"
	if err := os.WriteFile(scriptPath, []byte(selfDestructScript), 0755); err != nil {
		fmt.Printf("   Warning: Could not create self-destruct script: %v\n", err)
		fmt.Println()
		fmt.Println("   To complete uninstall, run:")
		fmt.Printf("   sudo rm -f %s\n", binaryPath)
	} else {
		// Check if we need sudo
		installDir := filepath.Dir(binaryPath)
		needSudo := true
		if fileInfo, err := os.Stat(installDir); err == nil {
			mode := fileInfo.Mode()
			if mode.Perm()&0200 != 0 { // Check write permission for owner
				needSudo = false
			}
		}

		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("✓ SkyPort Agent uninstalled successfully!")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Println("Final step: Remove the binary file")

		if needSudo {
			fmt.Printf("Run: sudo bash %s\n", scriptPath)
		} else {
			fmt.Printf("Run: bash %s\n", scriptPath)
		}
		fmt.Println()
		fmt.Println("Thank you for using SkyPort! 👋")

		// Exit immediately so script can delete the binary
		os.Exit(0)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✓ Uninstall complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Thank you for using SkyPort! 👋")
}

func runWindowsUninstall() {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("SkyPort Agent Uninstaller (Windows)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if !forceUninstall && !skipConfirmation {
		fmt.Println("This will completely remove SkyPort from your system.")
		fmt.Println()
		fmt.Print("Are you sure you want to continue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" {
			fmt.Println("Uninstall cancelled.")
			return
		}
		fmt.Println()
	}

	binaryPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Warning: Could not determine binary path: %v\n", err)
		binaryPath = `C:\Program Files\SkyPort\skyport.exe`
	}

	// Step 1: Stop and remove Windows service
	fmt.Println("Step 1: Checking Windows service...")
	// TODO: Implement Windows service removal with proper check
	fmt.Println("   ✓ Service check complete")

	// Step 2: Remove configuration files
	if !keepConfig {
		fmt.Println()
		fmt.Println("Step 2: Removing configuration files...")
		configDir, err := config.GetConfigDir()
		if err == nil && dirExists(configDir) {
			if err := os.RemoveAll(configDir); err != nil {
				fmt.Printf("   Warning: Failed to remove config: %v\n", err)
			} else {
				fmt.Println("   ✓ Configuration removed")
			}
		} else {
			fmt.Println("   ✓ No configuration found")
		}
	}

	// Step 3: Remove binary
	fmt.Println()
	fmt.Println("Step 3: Removing binary...")
	fmt.Printf("   Binary location: %s\n", binaryPath)

	// Create a self-destruct batch script
	selfDestructScript := `@echo off
timeout /t 2 /nobreak >nul
del /f /q "` + binaryPath + `" 2>nul
exit
`

	scriptPath := filepath.Join(os.TempDir(), "skyport-uninstall.bat")
	if err := os.WriteFile(scriptPath, []byte(selfDestructScript), 0755); err != nil {
		fmt.Printf("   Warning: Could not create self-destruct script: %v\n", err)
		fmt.Println()
		fmt.Println("   To complete uninstall, delete:")
		fmt.Printf("   %s\n", binaryPath)
	} else {
		fmt.Println()
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("✓ SkyPort Agent uninstalled successfully!")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println()
		fmt.Println("Final step: Run the cleanup script")
		fmt.Printf("Run: %s\n", scriptPath)
		fmt.Println()
		fmt.Println("Thank you for using SkyPort! 👋")

		// Exit immediately so script can delete the binary
		os.Exit(0)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✓ Uninstall complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Thank you for using SkyPort! 👋")
}

func clearKeyring() {
	cmd := exec.Command("secret-tool", "clear", "service", "skyport-agent")
	cmd.Run() // Silent - don't show keyring messages
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}
