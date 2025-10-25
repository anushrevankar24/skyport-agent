package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"skyport-agent/internal/config"
	"skyport-agent/internal/logger"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type TunnelManager struct {
	config        *config.Config
	activeTunnels map[string]*TunnelConnection
	mutex         sync.RWMutex
}

type TunnelConnection struct {
	Tunnel     config.Tunnel
	Connection *websocket.Conn
	Protocol   *AgentTunnelProtocol
	Context    context.Context
	Cancel     context.CancelFunc
	Status     string
}

func NewTunnelManager(cfg *config.Config) *TunnelManager {
	return &TunnelManager{
		config:        cfg,
		activeTunnels: make(map[string]*TunnelConnection),
	}
}

func (tm *TunnelManager) ConnectTunnel(tunnel *config.Tunnel, token string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Check if tunnel is already connected
	if _, exists := tm.activeTunnels[tunnel.ID]; exists {
		return fmt.Errorf("tunnel %s is already connected", tunnel.Name)
	}

	// Create connection context
	ctx, cancel := context.WithCancel(context.Background())

	// Connect to tunnel server - convert HTTP URL to WebSocket URL
	serverURL := strings.Replace(tm.config.ServerURL, "http://", "ws://", 1)
	serverURL = strings.Replace(serverURL, "https://", "wss://", 1)
	serverURL = serverURL + "/tunnel/connect"

	// Create headers with authentication
	headers := http.Header{}
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	headers.Add("X-Tunnel-ID", tunnel.ID)
	headers.Add("X-Tunnel-Auth", tunnel.AuthToken)

	// Create custom dialer with TCP keepalive enabled
	// This is critical for maintaining long-lived connections through NAT/firewalls
	dialer := &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			// Dial with timeout
			conn, err := net.DialTimeout(network, addr, 30*time.Second)
			if err != nil {
				return nil, err
			}

			// Enable TCP keepalive to maintain connection through NAT/firewalls
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				// Enable TCP keepalive
				if err := tcpConn.SetKeepAlive(true); err != nil {
					logger.Warning("Failed to enable TCP keepalive: %v", err)
				} else {
					// Send keepalive probes every 30 seconds
					// This keeps NAT/firewall entries alive and detects dead connections
					if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
						logger.Warning("Failed to set TCP keepalive period: %v", err)
					} else {
						logger.Debug("TCP keepalive enabled for tunnel %s (30s interval)", tunnel.Name)
					}
				}

				// Optional: Set TCP buffer sizes for better performance
				tcpConn.SetReadBuffer(64 * 1024)
				tcpConn.SetWriteBuffer(64 * 1024)
			}

			return conn, nil
		},
		HandshakeTimeout: 45 * time.Second,
		// Enable compression for better performance over slow connections
		EnableCompression: true,
	}

	// Connect WebSocket using custom dialer
	conn, _, err := dialer.Dial(serverURL, headers)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to connect to tunnel server: %w", err)
	}

	logger.Debug("Tunnel %s connected with TCP keepalive enabled", tunnel.Name)

	// Create tunnel protocol handler
	protocol := NewAgentTunnelProtocol(conn, tunnel.ID, tunnel.LocalPort)

	// Create tunnel connection
	tunnelConn := &TunnelConnection{
		Tunnel:     *tunnel,
		Connection: conn,
		Protocol:   protocol,
		Context:    ctx,
		Cancel:     cancel,
		Status:     "connected",
	}

	tm.activeTunnels[tunnel.ID] = tunnelConn

	// Start tunnel handler in background
	go tm.handleTunnelConnection(tunnelConn)

	return nil
}

// ConnectTunnelWithRetry connects a tunnel with automatic reconnection on failure
// This provides resilience against network interruptions and server restarts
func (tm *TunnelManager) ConnectTunnelWithRetry(tunnel *config.Tunnel, token string, autoReconnect bool) error {
	maxRetries := 5
	baseDelay := 2 * time.Second
	maxDelay := 60 * time.Second

	attempt := 0
	for {
		// Attempt to connect
		err := tm.ConnectTunnel(tunnel, token)
		if err == nil {
			logger.Debug("Tunnel %s connected successfully", tunnel.Name)

			// If auto-reconnect is enabled, monitor for disconnection and reconnect
			if autoReconnect {
				go tm.monitorAndReconnect(tunnel, token)
			}
			return nil
		}

		// Check if it's a "already connected" error
		if strings.Contains(err.Error(), "already connected") {
			return err
		}

		attempt++
		if attempt >= maxRetries && !autoReconnect {
			return fmt.Errorf("failed to connect tunnel after %d attempts: %w", maxRetries, err)
		}

		// Calculate exponential backoff delay
		multiplier := 1 << uint(attempt-1) // 2^(attempt-1)
		delay := time.Duration(int64(baseDelay) * int64(multiplier))
		if delay > maxDelay {
			delay = maxDelay
		}

		logger.Warning("Failed to connect tunnel %s (attempt %d): %v. Retrying in %v...",
			tunnel.Name, attempt, err, delay)

		// Wait before retrying
		time.Sleep(delay)

		// Reset attempt counter after max retries to continue trying with max delay
		if autoReconnect && attempt >= maxRetries {
			attempt = maxRetries - 1
		}
	}
}

// monitorAndReconnect monitors a tunnel connection and automatically reconnects if it disconnects
func (tm *TunnelManager) monitorAndReconnect(tunnel *config.Tunnel, token string) {
	checkInterval := 5 * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		// Check if tunnel is still connected
		if !tm.IsConnected(tunnel.ID) {
			logger.Warning("Tunnel %s disconnected, attempting to reconnect...", tunnel.Name)

			// Try to reconnect with exponential backoff
			baseDelay := 2 * time.Second
			maxDelay := 60 * time.Second
			attempt := 0
			maxReconnectAttempts := 10

			for attempt < maxReconnectAttempts {
				attempt++

				// Calculate exponential backoff delay
				multiplier := 1 << uint(attempt-1) // 2^(attempt-1)
				delay := time.Duration(int64(baseDelay) * int64(multiplier))
				if delay > maxDelay {
					delay = maxDelay
				}

				logger.Info("Reconnection attempt %d for tunnel %s...", attempt, tunnel.Name)

				err := tm.ConnectTunnel(tunnel, token)
				if err == nil {
					logger.Info("Tunnel %s reconnected successfully", tunnel.Name)
					return // Exit this goroutine, a new one will be started
				}

				if strings.Contains(err.Error(), "already connected") {
					logger.Debug("Tunnel %s is already connected", tunnel.Name)
					return
				}

				logger.Warning("Reconnection attempt %d failed for tunnel %s: %v. Retrying in %v...",
					attempt, tunnel.Name, err, delay)

				time.Sleep(delay)
			}

			logger.Error("Failed to reconnect tunnel %s after %d attempts. Giving up.",
				tunnel.Name, maxReconnectAttempts)
			return
		}
	}
}

func (tm *TunnelManager) DisconnectTunnel(tunnelID string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tunnelConn, exists := tm.activeTunnels[tunnelID]
	if !exists {
		return fmt.Errorf("tunnel not connected")
	}

	// Send WebSocket close frame for graceful shutdown
	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "User initiated shutdown")
	err := tunnelConn.Connection.WriteControl(
		websocket.CloseMessage,
		closeMessage,
		time.Now().Add(time.Second),
	)
	if err != nil {
		logger.Warning("Failed to send close frame for tunnel %s: %v", tunnelConn.Tunnel.Name, err)
	}

	// Give server time to acknowledge the close (100ms is enough)
	time.Sleep(100 * time.Millisecond)

	// Cancel context and close connection
	tunnelConn.Cancel()
	tunnelConn.Connection.Close()

	// Remove from active tunnels
	delete(tm.activeTunnels, tunnelID)

	return nil
}

func (tm *TunnelManager) GetTunnelStatus(tunnelID string) string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	if tunnelConn, exists := tm.activeTunnels[tunnelID]; exists {
		return tunnelConn.Status
	}
	return "disconnected"
}

func (tm *TunnelManager) IsConnected(tunnelID string) bool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	_, exists := tm.activeTunnels[tunnelID]
	return exists
}

func (tm *TunnelManager) GetActiveTunnels() []string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var tunnelIDs []string
	for id := range tm.activeTunnels {
		tunnelIDs = append(tunnelIDs, id)
	}
	return tunnelIDs
}

func (tm *TunnelManager) handleTunnelConnection(tunnelConn *TunnelConnection) {
	defer func() {
		// Cancel context first to stop all goroutines
		tunnelConn.Cancel()
		tm.mutex.Lock()
		delete(tm.activeTunnels, tunnelConn.Tunnel.ID)
		tm.mutex.Unlock()
		tunnelConn.Connection.Close()
		logger.Debug("Tunnel %s connection handler cleaned up", tunnelConn.Tunnel.Name)
	}()

	// Set up pong handler to extend read deadline when server responds to our pings
	tunnelConn.Connection.SetPongHandler(func(appData string) error {
		// Extend read deadline by 60 seconds (allowing for 4 missed pings at 15s intervals)
		tunnelConn.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Set initial read deadline (60 seconds allows time for first ping/pong exchange)
	if err := tunnelConn.Connection.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		logger.Error("Failed to set initial read deadline for tunnel %s: %v", tunnelConn.Tunnel.Name, err)
		return
	}

	// Send heartbeat periodically using WebSocket control frame pings
	go tm.sendHeartbeat(tunnelConn)

	for {
		select {
		case <-tunnelConn.Context.Done():
			return
		default:
			// Read message from server
			_, message, err := tunnelConn.Connection.ReadMessage()
			if err != nil {
				// Log the actual error that caused disconnect
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					logger.Debug("Tunnel %s closed gracefully: %v", tunnelConn.Tunnel.Name, err)
				} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.Debug("Tunnel %s unexpected close: %v", tunnelConn.Tunnel.Name, err)
				} else {
					// Connection errors during Ctrl+C or network issues - debug only
					logger.Debug("Tunnel %s connection error: %v", tunnelConn.Tunnel.Name, err)
				}
				tunnelConn.Status = "error"
				return
			}

			// Extend read deadline on successful read (application-level messages)
			tunnelConn.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))

			// Handle tunnel protocol messages
			go func() {
				if err := tunnelConn.Protocol.HandleTunnelMessage(message); err != nil {
					logger.Debug("Failed to handle tunnel message: %v", err)
					tunnelConn.Status = "error"
				}
			}()
		}
	}
}

func (tm *TunnelManager) sendHeartbeat(tunnelConn *TunnelConnection) {
	ticker := time.NewTicker(15 * time.Second) // Send heartbeat every 15 seconds
	defer ticker.Stop()

	for {
		select {
		case <-tunnelConn.Context.Done():
			return
		case <-ticker.C:
			// Use WebSocket control frame ping instead of JSON message
			// This is more efficient and properly integrated with the WebSocket protocol
			err := tunnelConn.Connection.WriteControl(
				websocket.PingMessage,
				[]byte{},
				time.Now().Add(10*time.Second),
			)
			if err != nil {
				logger.Error("Failed to send heartbeat for tunnel %s: %v", tunnelConn.Tunnel.Name, err)
				tunnelConn.Status = "error"
				tunnelConn.Cancel() // Cancel context to trigger cleanup
				return
			}
		}
	}
}
