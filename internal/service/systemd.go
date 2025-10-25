package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SystemdService manages systemd service integration
type SystemdService struct {
	serviceName string
	user        string
	execPath    string
	configPath  string
}

// NewSystemdService creates a new systemd service manager
func NewSystemdService() *SystemdService {
	// Get the actual user, even when running with sudo
	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	if user == "" || user == "root" {
		user = "root"
	}

	execPath, _ := os.Executable()

	// Get home directory for the actual user (not root)
	var configPath string
	if user != "root" {
		configPath = filepath.Join("/home", user, ".skyport")
	} else {
		configPath = filepath.Join(os.Getenv("HOME"), ".skyport")
	}

	return &SystemdService{
		serviceName: "skyport-agent",
		user:        user,
		execPath:    execPath,
		configPath:  configPath,
	}
}

// Install installs the systemd service
func (s *SystemdService) Install() error {
	serviceContent := s.generateServiceFile()
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", s.serviceName)

	// Write service file
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if err := exec.Command("systemctl", "enable", s.serviceName).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

// Uninstall removes the systemd service
func (s *SystemdService) Uninstall() error {
	// Stop service first
	exec.Command("systemctl", "stop", s.serviceName).Run()

	// Disable service
	exec.Command("systemctl", "disable", s.serviceName).Run()

	// Remove service file
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", s.serviceName)
	os.Remove(servicePath)

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	return nil
}

// Start starts the systemd service
func (s *SystemdService) Start() error {
	return exec.Command("systemctl", "start", s.serviceName).Run()
}

// Stop stops the systemd service
func (s *SystemdService) Stop() error {
	return exec.Command("systemctl", "stop", s.serviceName).Run()
}

// Restart restarts the systemd service
func (s *SystemdService) Restart() error {
	return exec.Command("systemctl", "restart", s.serviceName).Run()
}

// Status returns the service status
func (s *SystemdService) Status() (string, error) {
	cmd := exec.Command("systemctl", "is-active", s.serviceName)
	output, err := cmd.Output()
	if err != nil {
		return "inactive", err
	}
	return strings.TrimSpace(string(output)), nil
}

// IsInstalled checks if the service is installed
func (s *SystemdService) IsInstalled() bool {
	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", s.serviceName)
	_, err := os.Stat(servicePath)
	return err == nil
}

// IsRunning checks if the service is running
func (s *SystemdService) IsRunning() bool {
	status, _ := s.Status()
	return status == "active"
}

// GetLogs returns recent service logs
func (s *SystemdService) GetLogs(lines int) (string, error) {
	cmd := exec.Command("journalctl", "-u", s.serviceName, "-n", strconv.Itoa(lines), "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// generateServiceFile generates the systemd service file content
func (s *SystemdService) generateServiceFile() string {
	return fmt.Sprintf(`[Unit]
Description=SkyPort Agent - Secure tunnel client
Documentation=https://github.com/your-org/skyport
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s daemon
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3

# Environment
Environment=SKYPORT_CONFIG_DIR=%s
Environment=SKYPORT_LOG_LEVEL=info

# Security
NoNewPrivileges=true
PrivateTmp=true
ReadWritePaths=%s

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=skyport-agent

[Install]
WantedBy=multi-user.target
`, s.user, s.user, s.execPath, s.configPath, s.configPath)
}
