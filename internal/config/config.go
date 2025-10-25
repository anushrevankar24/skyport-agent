package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// These variables will be set at build time using -ldflags
var (
	DefaultServerURL    = "http://localhost:8080/api/v1"
	DefaultWebURL       = "http://localhost:3000"
	DefaultTunnelDomain = "localhost:8080"
	DebugMode           = "true" // "true" or "false" as string (set at build time)
)

// Config represents the application configuration
type Config struct {
	ServerURL    string `json:"server_url"`
	WebURL       string `json:"web_url"`
	TunnelDomain string `json:"tunnel_domain"`
}

// UserData represents user authentication data
type UserData struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

// Load returns the application configuration
// It first checks environment variables, then falls back to build-time defaults
func Load() *Config {
	return &Config{
		ServerURL:    getEnv("SKYPORT_SERVER_URL", DefaultServerURL),
		WebURL:       getEnv("SKYPORT_WEB_URL", DefaultWebURL),
		TunnelDomain: getEnv("SKYPORT_TUNNEL_DOMAIN", DefaultTunnelDomain),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// AppConfig represents the agent configuration
type AppConfig struct {
	UserToken string             `json:"user_token"`
	Tunnels   map[string]*Tunnel `json:"tunnels"`
	LastSync  time.Time          `json:"last_sync"`
}

// Tunnel represents a tunnel configuration
type Tunnel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	LocalPort int    `json:"local_port"`
	AuthToken string `json:"auth_token"`
	IsActive  bool   `json:"is_active"`
	AutoStart bool   `json:"auto_start"` // Auto-connect when agent starts
}

// ConfigManager handles the agent configuration
type ConfigManager struct {
	configFile string
}

// NewConfigManager creates a new config manager
func NewConfigManager() *ConfigManager {
	configDir := getConfigDir()
	return &ConfigManager{
		configFile: filepath.Join(configDir, "skyport.json"),
	}
}

// getConfigDir returns platform-specific config directory
func getConfigDir() string {
	var configDir string

	// Use standard app data directories
	if home, err := os.UserHomeDir(); err == nil {
		configDir = filepath.Join(home, ".skyport")
	} else {
		configDir = "."
	}

	// Create directory if it doesn't exist
	os.MkdirAll(configDir, 0755)
	return configDir
}

// LoadConfig loads the application configuration
func (cm *ConfigManager) LoadConfig() (*AppConfig, error) {
	if _, err := os.Stat(cm.configFile); os.IsNotExist(err) {
		// Return empty config if file doesn't exist
		return &AppConfig{
			Tunnels:  make(map[string]*Tunnel),
			LastSync: time.Now(),
		}, nil
	}

	data, err := os.ReadFile(cm.configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if config.Tunnels == nil {
		config.Tunnels = make(map[string]*Tunnel)
	}

	return &config, nil
}

// SaveConfig saves the application configuration
func (cm *ConfigManager) SaveConfig(config *AppConfig) error {
	config.LastSync = time.Now()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(cm.configFile, data, 0644)
}

// SaveUserToken saves the user's authentication token
func (cm *ConfigManager) SaveUserToken(token string) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	config.UserToken = token
	return cm.SaveConfig(config)
}

// GetUserToken gets the user's authentication token
func (cm *ConfigManager) GetUserToken() (string, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return "", err
	}

	return config.UserToken, nil
}

// SetTunnelAutoStart enables/disables auto-start for a tunnel
func (cm *ConfigManager) SetTunnelAutoStart(tunnelID string, autoStart bool) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	if tunnel, exists := config.Tunnels[tunnelID]; exists {
		tunnel.AutoStart = autoStart
		return cm.SaveConfig(config)
	}

	return fmt.Errorf("tunnel %s not found", tunnelID)
}

// SetTunnelActive updates tunnel active status
func (cm *ConfigManager) SetTunnelActive(tunnelID string, isActive bool) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	if tunnel, exists := config.Tunnels[tunnelID]; exists {
		tunnel.IsActive = isActive
		return cm.SaveConfig(config)
	}

	return fmt.Errorf("tunnel %s not found", tunnelID)
}

// GetAutoStartTunnels returns tunnels that should auto-start
func (cm *ConfigManager) GetAutoStartTunnels() ([]*Tunnel, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return nil, err
	}

	var autoStartTunnels []*Tunnel
	for _, tunnel := range config.Tunnels {
		if tunnel.AutoStart {
			autoStartTunnels = append(autoStartTunnels, tunnel)
		}
	}

	return autoStartTunnels, nil
}

// SaveUserData saves user data to disk
func SaveUserData(userData *UserData) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "user.json")
	data, err := json.MarshalIndent(userData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}

// LoadUserData loads user data from disk
func LoadUserData() (*UserData, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(configDir, "user.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var userData UserData
	if err := json.Unmarshal(data, &userData); err != nil {
		return nil, err
	}

	return &userData, nil
}

// ClearUserData removes user data from disk
func ClearUserData() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	configFile := filepath.Join(configDir, "user.json")
	return os.Remove(configFile)
}

// GetConfigDir returns the configuration directory
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".skyport")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return configDir, nil
}

// IsDebugMode returns true if debug mode is enabled
func IsDebugMode() bool {
	return DebugMode == "true"
}
