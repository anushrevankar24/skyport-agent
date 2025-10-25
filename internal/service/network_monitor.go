package service

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// NetworkMonitor detects network changes and triggers reconnections
type NetworkMonitor struct {
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	lastIP        string
	lastInterface string
	changeChan    chan NetworkChange
	monitoring    bool
}

// NetworkChange represents a network change event
type NetworkChange struct {
	Type        string    `json:"type"`
	OldValue    string    `json:"old_value"`
	NewValue    string    `json:"new_value"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
}

// NewNetworkMonitor creates a new network monitor
func NewNetworkMonitor() *NetworkMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	return &NetworkMonitor{
		ctx:        ctx,
		cancel:     cancel,
		changeChan: make(chan NetworkChange, 10),
	}
}

// Start begins network monitoring
func (nm *NetworkMonitor) Start() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.monitoring {
		return
	}

	nm.monitoring = true

	// Get initial network state
	nm.updateNetworkState()

	// Start monitoring goroutine
	go nm.monitorLoop()

	log.Println("Network monitor started")
}

// Stop stops network monitoring
func (nm *NetworkMonitor) Stop() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if !nm.monitoring {
		return
	}

	nm.monitoring = false
	nm.cancel()
	close(nm.changeChan)

	log.Println("Network monitor stopped")
}

// GetChangeChannel returns the network change channel
func (nm *NetworkMonitor) GetChangeChannel() <-chan NetworkChange {
	return nm.changeChan
}

// monitorLoop continuously monitors network changes
func (nm *NetworkMonitor) monitorLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			nm.checkNetworkChanges()
		}
	}
}

// checkNetworkChanges checks for network changes
func (nm *NetworkMonitor) checkNetworkChanges() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if !nm.monitoring {
		return
	}

	// Get current network state
	currentIP, currentInterface := nm.getCurrentNetworkState()

	// Check for IP address changes
	if nm.lastIP != "" && nm.lastIP != currentIP {
		change := NetworkChange{
			Type:        "ip_change",
			OldValue:    nm.lastIP,
			NewValue:    currentIP,
			Timestamp:   time.Now(),
			Description: fmt.Sprintf("IP address changed from %s to %s", nm.lastIP, currentIP),
		}

		select {
		case nm.changeChan <- change:
			log.Printf("Network change detected: %s", change.Description)
		default:
			log.Printf("Network change channel full, dropping change: %s", change.Description)
		}
	}

	// Check for interface changes
	if nm.lastInterface != "" && nm.lastInterface != currentInterface {
		change := NetworkChange{
			Type:        "interface_change",
			OldValue:    nm.lastInterface,
			NewValue:    currentInterface,
			Timestamp:   time.Now(),
			Description: fmt.Sprintf("Network interface changed from %s to %s", nm.lastInterface, currentInterface),
		}

		select {
		case nm.changeChan <- change:
			log.Printf("Network change detected: %s", change.Description)
		default:
			log.Printf("Network change channel full, dropping change: %s", change.Description)
		}
	}

	// Update stored state
	nm.lastIP = currentIP
	nm.lastInterface = currentInterface
}

// updateNetworkState updates the stored network state
func (nm *NetworkMonitor) updateNetworkState() {
	nm.lastIP, nm.lastInterface = nm.getCurrentNetworkState()
}

// getCurrentNetworkState gets the current network state
func (nm *NetworkMonitor) getCurrentNetworkState() (string, string) {
	// Get the primary network interface
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Error getting network interfaces: %v", err)
		return "", ""
	}

	var primaryIP string
	var primaryInterface string

	for _, iface := range interfaces {
		// Skip loopback and inactive interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil { // IPv4
					primaryIP = ipNet.IP.String()
					primaryInterface = iface.Name
					break
				}
			}
		}

		if primaryIP != "" {
			break
		}
	}

	return primaryIP, primaryInterface
}

// GetCurrentNetworkInfo returns detailed network information
func (nm *NetworkMonitor) GetCurrentNetworkInfo() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	ip, interfaceName := nm.getCurrentNetworkState()

	return map[string]interface{}{
		"current_ip":        ip,
		"current_interface": interfaceName,
		"last_ip":           nm.lastIP,
		"last_interface":    nm.lastInterface,
		"monitoring":        nm.monitoring,
	}
}

// TestConnectivity tests network connectivity to various endpoints
func (nm *NetworkMonitor) TestConnectivity() map[string]bool {
	endpoints := []string{
		"google.com:80",
		"github.com:443",
		"1.1.1.1:53",
		"8.8.8.8:53",
	}

	results := make(map[string]bool)

	for _, endpoint := range endpoints {
		conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
		if err != nil {
			results[endpoint] = false
		} else {
			conn.Close()
			results[endpoint] = true
		}
	}

	return results
}

// WaitForNetwork waits for network connectivity to be restored
func (nm *NetworkMonitor) WaitForNetwork(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		connectivity := nm.TestConnectivity()

		// Check if any endpoint is reachable
		for _, reachable := range connectivity {
			if reachable {
				return true
			}
		}

		time.Sleep(5 * time.Second)
	}

	return false
}
