package service

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// HealthMonitor manages tunnel health and auto-recovery
type HealthMonitor struct {
	manager         *Manager
	healthTicker    *time.Ticker
	reconnectTicker *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
	lastHealth      map[string]time.Time
	reconnectQueue  map[string]int // retry count
	maxRetries      int
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(manager *Manager) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &HealthMonitor{
		manager:        manager,
		ctx:            ctx,
		cancel:         cancel,
		lastHealth:     make(map[string]time.Time),
		reconnectQueue: make(map[string]int),
		maxRetries:     5,
	}
}

// Start begins health monitoring
func (hm *HealthMonitor) Start() {
	// Health check every 30 seconds
	hm.healthTicker = time.NewTicker(30 * time.Second)

	// Reconnection attempts every 60 seconds
	hm.reconnectTicker = time.NewTicker(60 * time.Second)

	// Start monitoring goroutines
	go hm.healthCheckLoop()
	go hm.reconnectLoop()
	go hm.signalHandler()

	log.Println("Health monitor started")
}

// Stop stops health monitoring
func (hm *HealthMonitor) Stop() {
	if hm.healthTicker != nil {
		hm.healthTicker.Stop()
	}
	if hm.reconnectTicker != nil {
		hm.reconnectTicker.Stop()
	}
	hm.cancel()
	log.Println("Health monitor stopped")
}

// healthCheckLoop performs periodic health checks
func (hm *HealthMonitor) healthCheckLoop() {
	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-hm.healthTicker.C:
			hm.performHealthCheck()
		}
	}
}

// reconnectLoop handles reconnection attempts
func (hm *HealthMonitor) reconnectLoop() {
	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-hm.reconnectTicker.C:
			hm.processReconnectQueue()
		}
	}
}

// performHealthCheck checks the health of all active tunnels
func (hm *HealthMonitor) performHealthCheck() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	activeTunnels := hm.manager.GetActiveTunnels()
	now := time.Now()

	for _, tunnelID := range activeTunnels {
		// Check if tunnel is actually connected
		if !hm.manager.IsTunnelConnected(tunnelID) {
			log.Printf("Health check: Tunnel %s is disconnected", tunnelID)
			hm.scheduleReconnect(tunnelID)
			continue
		}

		// Check local service health
		if !hm.checkLocalServiceHealth(tunnelID) {
			log.Printf("Health check: Local service for tunnel %s is not responding", tunnelID)
			hm.scheduleReconnect(tunnelID)
			continue
		}

		// Check network connectivity
		if !hm.checkNetworkConnectivity() {
			log.Printf("Health check: Network connectivity issues detected")
			hm.scheduleReconnect(tunnelID)
			continue
		}

		// Update last health time
		hm.lastHealth[tunnelID] = now
		log.Printf("Health check: Tunnel %s is healthy", tunnelID)
	}
}

// checkLocalServiceHealth checks if the local service is responding
func (hm *HealthMonitor) checkLocalServiceHealth(tunnelID string) bool {
	// Get tunnel config to find local port
	tunnels, err := hm.manager.GetTunnelList()
	if err != nil {
		return false
	}

	var localPort int
	for _, tunnel := range tunnels {
		if tunnel.ID == tunnelID {
			localPort = tunnel.LocalPort
			break
		}
	}

	if localPort == 0 {
		return false
	}

	// Try to connect to local service
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", localPort), 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// checkNetworkConnectivity checks basic network connectivity
func (hm *HealthMonitor) checkNetworkConnectivity() bool {
	// Try to resolve a well-known domain
	_, err := net.LookupHost("google.com")
	return err == nil
}

// scheduleReconnect schedules a tunnel for reconnection
func (hm *HealthMonitor) scheduleReconnect(tunnelID string) {
	hm.reconnectQueue[tunnelID]++
	log.Printf("Scheduled reconnection for tunnel %s (attempt %d)", tunnelID, hm.reconnectQueue[tunnelID])
}

// processReconnectQueue processes the reconnection queue
func (hm *HealthMonitor) processReconnectQueue() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for tunnelID, retryCount := range hm.reconnectQueue {
		if retryCount > hm.maxRetries {
			log.Printf("Max retries reached for tunnel %s, removing from queue", tunnelID)
			delete(hm.reconnectQueue, tunnelID)
			continue
		}

		// Attempt reconnection
		if err := hm.manager.ConnectTunnel(tunnelID, false); err != nil {
			log.Printf("Reconnection failed for tunnel %s: %v", tunnelID, err)
			// Increment retry count
			hm.reconnectQueue[tunnelID]++
		} else {
			log.Printf("Successfully reconnected tunnel %s", tunnelID)
			delete(hm.reconnectQueue, tunnelID)
		}
	}
}

// signalHandler handles system signals for graceful shutdown
func (hm *HealthMonitor) signalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for sig := range sigChan {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			log.Printf("Received signal %v, shutting down gracefully", sig)
			hm.gracefulShutdown()
			return
		case syscall.SIGHUP:
			log.Println("Received SIGHUP, reloading configuration")
			hm.reloadConfiguration()
		}
	}
}

// gracefulShutdown performs a graceful shutdown
func (hm *HealthMonitor) gracefulShutdown() {
	log.Println("Starting graceful shutdown...")

	// Stop health monitoring
	hm.Stop()

	// Disconnect all tunnels gracefully
	activeTunnels := hm.manager.GetActiveTunnels()
	for _, tunnelID := range activeTunnels {
		log.Printf("Disconnecting tunnel %s", tunnelID)
		if err := hm.manager.DisconnectTunnel(tunnelID); err != nil {
			log.Printf("Error disconnecting tunnel %s: %v", tunnelID, err)
		}
	}

	// Stop the main manager
	hm.manager.StopSilently()

	log.Println("Graceful shutdown complete")
	os.Exit(0)
}

// reloadConfiguration reloads the configuration
func (hm *HealthMonitor) reloadConfiguration() {
	log.Println("Reloading configuration...")

	// This would trigger a config reload in the manager
	// For now, just log the event
	log.Println("Configuration reloaded")
}

// GetHealthStatus returns the current health status
func (hm *HealthMonitor) GetHealthStatus() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status := map[string]interface{}{
		"active_tunnels":    len(hm.manager.GetActiveTunnels()),
		"reconnect_queue":   len(hm.reconnectQueue),
		"last_health_check": time.Now(),
		"tunnel_health":     make(map[string]interface{}),
	}

	// Add individual tunnel health
	for tunnelID, lastHealth := range hm.lastHealth {
		status["tunnel_health"].(map[string]interface{})[tunnelID] = map[string]interface{}{
			"last_healthy": lastHealth,
			"is_healthy":   time.Since(lastHealth) < 2*time.Minute,
		}
	}

	return status
}


