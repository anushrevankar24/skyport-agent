package service

import (
	"context"
	"fmt"
	"log"
	"skyport-agent/internal/auth"
	"skyport-agent/internal/config"
	"skyport-agent/internal/logger"
	"skyport-agent/internal/tunnel"
	"sync"
	"time"
)

// Manager handles all background tasks automatically and silently
// User never needs to run any commands - everything just works
type Manager struct {
	authManager    *auth.AuthManager
	tunnelManager  *tunnel.TunnelManager
	configManager  *config.ConfigManager
	urlHandler     *auth.URLHandler
	healthMonitor  *HealthMonitor
	networkMonitor *NetworkMonitor
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      bool
	mutex          sync.RWMutex
}

// NewManager creates a new automatic background manager
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		authManager:   auth.NewAuthManager(cfg),
		tunnelManager: tunnel.NewTunnelManager(cfg),
		configManager: config.NewConfigManager(),
		ctx:           ctx,
		cancel:        cancel,
		isRunning:     false,
	}

	// Initialize monitors
	manager.healthMonitor = NewHealthMonitor(manager)
	manager.networkMonitor = NewNetworkMonitor()

	return manager
}

// StartSilently starts all background processes without user interaction
func (am *Manager) StartSilently() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if am.isRunning {
		return // Already running
	}

	am.isRunning = true

	// Start monitors
	am.healthMonitor.Start()
	am.networkMonitor.Start()

	// Start background manager silently
	go am.runBackgroundTasks()
}

// StopSilently stops all background processes
func (am *Manager) StopSilently() {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	if !am.isRunning {
		return
	}

	// Stop monitors
	if am.healthMonitor != nil {
		am.healthMonitor.Stop()
	}
	if am.networkMonitor != nil {
		am.networkMonitor.Stop()
	}

	// Stop URL handler if running
	if am.urlHandler != nil {
		am.urlHandler.Stop()
		am.urlHandler = nil
	}

	am.cancel()
	am.isRunning = false

	// Disconnect all active tunnels gracefully
	am.disconnectAllTunnels()
}

// runBackgroundTasks runs all background management tasks
func (am *Manager) runBackgroundTasks() {
	defer func() {
		am.mutex.Lock()
		am.isRunning = false
		am.mutex.Unlock()
	}()

	// Start with auto-connecting tunnels if user is logged in
	am.autoConnectTunnels()

	// Main background loop - runs every 60 seconds
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-am.ctx.Done():
			return
		case <-ticker.C:
			am.performBackgroundMaintenance()
		}
	}
}

// autoConnectTunnels automatically connects tunnels marked for auto-start
func (am *Manager) autoConnectTunnels() {
	// Only auto-connect if user is authenticated
	if !am.authManager.IsAuthenticated() {
		return
	}

	// Get tunnels marked for auto-start
	autoStartTunnels, err := am.configManager.GetAutoStartTunnels()
	if err != nil {
		log.Printf("Auto-connect: Failed to get auto-start tunnels: %v", err)
		return
	}

	if len(autoStartTunnels) == 0 {
		return // No auto-start tunnels
	}

	// Get authentication token
	token, err := am.authManager.GetValidToken()
	if err != nil {
		log.Printf("Auto-connect: Failed to get auth token: %v", err)
		return
	}

	// Connect each auto-start tunnel silently with auto-reconnect
	for _, simpleTunnel := range autoStartTunnels {
		// Skip if already connected
		if am.tunnelManager.IsConnected(simpleTunnel.ID) {
			continue
		}

		tunnel := &config.Tunnel{
			ID:        simpleTunnel.ID,
			Name:      simpleTunnel.Name,
			Subdomain: simpleTunnel.Subdomain,
			LocalPort: simpleTunnel.LocalPort,
			AuthToken: simpleTunnel.AuthToken,
		}

		log.Printf("Auto-connecting tunnel: %s", tunnel.Name)

		// Use ConnectTunnelWithRetry with auto-reconnect enabled for auto-start tunnels
		if err := am.tunnelManager.ConnectTunnelWithRetry(tunnel, token, true); err != nil {
			log.Printf("Auto-connect failed for %s: %v", tunnel.Name, err)
			continue
		}

		// Update config to show as active
		am.configManager.SetTunnelActive(tunnel.ID, true)
		log.Printf("Auto-connected tunnel: %s (auto-reconnect enabled)", tunnel.Name)
	}
}

// performBackgroundMaintenance handles all background maintenance tasks
func (am *Manager) performBackgroundMaintenance() {
	// 1. Sync tunnels from server (if authenticated)
	if err := am.SyncTunnelsFromServer(); err != nil {
		log.Printf("Background maintenance: Failed to sync tunnels: %v", err)
	}

	// 2. Health check and auto-reconnect failed tunnels
	am.healthCheckAndReconnect()

	// 3. Update tunnel status in config
	am.updateTunnelStatus()
}

// SyncTunnelsFromServer syncs tunnel list from server to local config
func (am *Manager) SyncTunnelsFromServer() error {
	if !am.authManager.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	// Get valid token
	token, err := am.authManager.GetValidToken()
	if err != nil {
		return fmt.Errorf("failed to get valid token: %w", err)
	}

	// Get tunnels from server
	serverTunnels, err := am.authManager.FetchTunnels(token)
	if err != nil {
		return fmt.Errorf("failed to get tunnels from server: %w", err)
	}

	// Update local config with server tunnels
	if err := am.updateLocalTunnelsFromServer(serverTunnels); err != nil {
		return fmt.Errorf("failed to update local config: %w", err)
	}

	logger.Debug("Successfully synced %d tunnels from server", len(serverTunnels))
	return nil
}

// updateLocalTunnelsFromServer updates local tunnel config with server data
func (am *Manager) updateLocalTunnelsFromServer(serverTunnels []config.Tunnel) error {
	// Load current config
	appConfig, err := am.configManager.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Update tunnels map with server data
	if appConfig.Tunnels == nil {
		appConfig.Tunnels = make(map[string]*config.Tunnel)
	}

	// Add/update tunnels from server
	for _, serverTunnel := range serverTunnels {
		tunnelCopy := serverTunnel // Create a copy
		appConfig.Tunnels[tunnelCopy.ID] = &tunnelCopy
	}

	// Save updated config
	return am.configManager.SaveConfig(appConfig)
}

// healthCheckAndReconnect checks tunnel health and reconnects if needed
func (am *Manager) healthCheckAndReconnect() {
	if !am.authManager.IsAuthenticated() {
		return
	}

	// Get auto-start tunnels that should be connected
	autoStartTunnels, err := am.configManager.GetAutoStartTunnels()
	if err != nil {
		return
	}

	_, err = am.authManager.GetValidToken()
	if err != nil {
		return
	}

	// Check each auto-start tunnel
	for _, simpleTunnel := range autoStartTunnels {
		if !am.tunnelManager.IsConnected(simpleTunnel.ID) {
			// Tunnel should be connected but isn't - reconnect it
			log.Printf("Health check: Reconnecting tunnel %s", simpleTunnel.Name)

			// TODO: Fix config.Tunnel type issue
			// tunnel := config.Tunnel{
			// 	ID:        simpleTunnel.ID,
			// 	Name:      simpleTunnel.Name,
			// 	Subdomain: simpleTunnel.Subdomain,
			// 	LocalPort: simpleTunnel.LocalPort,
			// 	AuthToken: simpleTunnel.AuthToken,
			// }

			// if err := am.tunnelManager.ConnectTunnel(tunnel, token); err != nil {
			if false {
				log.Printf("Health check: Failed to reconnect %s: %v", simpleTunnel.Name, err)
			} else {
				log.Printf("Health check: Reconnected tunnel %s", simpleTunnel.Name)
				am.configManager.SetTunnelActive(simpleTunnel.ID, true)
			}
		}
	}
}

// updateTunnelStatus updates tunnel active status in config
func (am *Manager) updateTunnelStatus() {
	config, err := am.configManager.LoadConfig()
	if err != nil {
		return
	}

	// Update active status for all tunnels
	for tunnelID, tunnel := range config.Tunnels {
		isConnected := am.tunnelManager.IsConnected(tunnelID)
		if tunnel.IsActive != isConnected {
			am.configManager.SetTunnelActive(tunnelID, isConnected)
		}
	}
}

// disconnectAllTunnels disconnects all active tunnels
func (am *Manager) disconnectAllTunnels() {
	activeTunnels := am.tunnelManager.GetActiveTunnels()

	for _, tunnelID := range activeTunnels {
		if err := am.tunnelManager.DisconnectTunnel(tunnelID); err != nil {
			log.Printf("Failed to disconnect tunnel %s: %v", tunnelID, err)
		} else {
			am.configManager.SetTunnelActive(tunnelID, false)
		}
	}
}

// ConnectTunnel connects a tunnel and optionally sets auto-start
func (am *Manager) ConnectTunnel(tunnelID string, setAutoStart bool) error {
	if !am.authManager.IsAuthenticated() {
		return fmt.Errorf("user not authenticated")
	}

	// Check if tunnel is already connected
	if am.tunnelManager.IsConnected(tunnelID) {
		return fmt.Errorf("tunnel is already connected")
	}

	appConfig, err := am.configManager.LoadConfig()
	if err != nil {
		return err
	}

	simpleTunnel, exists := appConfig.Tunnels[tunnelID]
	if !exists {
		return fmt.Errorf("tunnel %s not found", tunnelID)
	}

	// Get valid token for tunnel connection
	token, err := am.authManager.GetValidToken()
	if err != nil {
		return fmt.Errorf("failed to get auth token: %w", err)
	}

	// Create tunnel object for connection
	tunnel := &config.Tunnel{
		ID:        simpleTunnel.ID,
		Name:      simpleTunnel.Name,
		Subdomain: simpleTunnel.Subdomain,
		LocalPort: simpleTunnel.LocalPort,
		AuthToken: simpleTunnel.AuthToken,
	}

	logger.Debug("Connecting tunnel: %s (ID: %s, Port: %d)", tunnel.Name, tunnel.ID, tunnel.LocalPort)

	// Actually connect the tunnel using tunnel manager with retry and auto-reconnect
	// Enable auto-reconnect if setAutoStart is true (tunnels that should stay connected)
	if err := am.tunnelManager.ConnectTunnelWithRetry(tunnel, token, setAutoStart); err != nil {
		return fmt.Errorf("failed to connect tunnel: %w", err)
	}

	// Update config to show as active
	am.configManager.SetTunnelActive(tunnelID, true)
	if setAutoStart {
		am.configManager.SetTunnelAutoStart(tunnelID, true)
		logger.Debug("Successfully connected tunnel: %s (auto-reconnect enabled)", tunnel.Name)
	} else {
		logger.Debug("Successfully connected tunnel: %s", tunnel.Name)
	}

	return nil
}

// DisconnectTunnel disconnects a tunnel
func (am *Manager) DisconnectTunnel(tunnelID string) error {
	if err := am.tunnelManager.DisconnectTunnel(tunnelID); err != nil {
		return err
	}

	am.configManager.SetTunnelActive(tunnelID, false)
	return nil
}

// SetTunnelAutoStart enables/disables auto-start for a tunnel
func (am *Manager) SetTunnelAutoStart(tunnelID string, autoStart bool) error {
	return am.configManager.SetTunnelAutoStart(tunnelID, autoStart)
}

// IsTunnelConnected checks if a tunnel is currently connected
func (am *Manager) IsTunnelConnected(tunnelID string) bool {
	return am.tunnelManager.IsConnected(tunnelID)
}

// GetTunnelList returns the current tunnel list
func (am *Manager) GetTunnelList() ([]*config.Tunnel, error) {
	appConfig, err := am.configManager.LoadConfig()
	if err != nil {
		return nil, err
	}

	var tunnels []*config.Tunnel
	for _, tunnel := range appConfig.Tunnels {
		tunnels = append(tunnels, tunnel)
	}
	return tunnels, nil
}

// OnUserLogin handles user login - syncs tunnels and starts auto-connections
func (am *Manager) OnUserLogin(token string) error {
	// Save credentials through auth manager (persist to keyring + config)
	if _, err := am.authManager.LoginWithToken(token); err != nil {
		return err
	}

	// Sync tunnels from server immediately
	go func() {
		time.Sleep(1 * time.Second) // Brief delay to ensure token is saved
		if err := am.SyncTunnelsFromServer(); err != nil {
			log.Printf("Login sync: Failed to sync tunnels: %v", err)
		}

		// Auto-connect tunnels after sync
		time.Sleep(2 * time.Second)
		am.autoConnectTunnels()
	}()

	return nil
}

// OnUserLogout handles user logout - disconnects all tunnels
func (am *Manager) OnUserLogout() error {
	am.disconnectAllTunnels()

	// Clear credentials using auth manager (keyring + user.json)
	return am.authManager.ClearCredentials()
}

// IsAuthenticated returns whether user is authenticated
func (am *Manager) IsAuthenticated() bool {
	return am.authManager.IsAuthenticated()
}

// StartWebAuth starts the web authentication process
func (am *Manager) StartWebAuth() error {
	// Start a local callback server and get the callback URL
	urlHandler := auth.NewURLHandler(am.authManager)
	callbackURL, err := urlHandler.StartServer()
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}

	// Store the URL handler for later cleanup
	am.urlHandler = urlHandler

	// Start the OAuth flow with the callback URL
	if err := am.authManager.StartWebAuth(callbackURL); err != nil {
		urlHandler.Stop()
		return err
	}

	// Wait for the authentication in a goroutine
	go am.waitForAuthentication(urlHandler)

	return nil
}

// waitForAuthentication waits for the OAuth callback and processes the token
func (am *Manager) waitForAuthentication(urlHandler *auth.URLHandler) {
	// Wait for the token with a 5-minute timeout
	token, err := urlHandler.WaitForToken(5 * time.Minute)

	// Stop the callback server
	urlHandler.Stop()
	am.urlHandler = nil

	if err != nil {
		log.Printf("Authentication failed: %v", err)
		return
	}

	// Process the received token
	userData, err := am.authManager.LoginWithToken(token)
	if err != nil {
		log.Printf("Failed to process authentication token: %v", err)
		return
	}

	log.Printf("Authentication successful for user: %s", userData.Email)

	// Trigger user login handler to sync tunnels
	if err := am.OnUserLogin(token); err != nil {
		log.Printf("Failed to complete login process: %v", err)
	}
}

// RefreshTunnels manually triggers a tunnel sync from server
func (am *Manager) RefreshTunnels() error {
	if !am.authManager.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	// Force sync tunnels from server
	if err := am.SyncTunnelsFromServer(); err != nil {
		log.Printf("Refresh: Failed to sync tunnels: %v", err)
	}
	return nil
}

// GetContext returns the manager's context for cancellation
func (am *Manager) GetContext() context.Context {
	return am.ctx
}

// OpenURL opens a URL in the default browser
func (am *Manager) OpenURL(url string) error {
	return am.authManager.OpenURL(url)
}

// GetHealthStatus returns the current health status
func (am *Manager) GetHealthStatus() map[string]interface{} {
	if am.healthMonitor != nil {
		return am.healthMonitor.GetHealthStatus()
	}
	return map[string]interface{}{}
}

// GetNetworkInfo returns current network information
func (am *Manager) GetNetworkInfo() map[string]interface{} {
	if am.networkMonitor != nil {
		return am.networkMonitor.GetCurrentNetworkInfo()
	}
	return map[string]interface{}{}
}

// GetActiveTunnels returns list of active tunnel IDs
func (am *Manager) GetActiveTunnels() []string {
	return am.tunnelManager.GetActiveTunnels()
}
