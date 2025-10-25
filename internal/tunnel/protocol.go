package tunnel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"skyport-agent/internal/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TunnelMessage represents a message in the tunnel protocol
type TunnelMessage struct {
	Type      string            `json:"type"`
	ID        string            `json:"id"`
	Method    string            `json:"method,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      []byte            `json:"body,omitempty"`
	Status    int               `json:"status,omitempty"`
	Error     string            `json:"error,omitempty"`
	Timestamp int64             `json:"timestamp"`
}

// AgentTunnelProtocol handles the agent side of tunnel protocol
type AgentTunnelProtocol struct {
	conn       *websocket.Conn
	localPort  int
	tunnelID   string
	writeMutex sync.Mutex
}

func NewAgentTunnelProtocol(conn *websocket.Conn, tunnelID string, localPort int) *AgentTunnelProtocol {
	return &AgentTunnelProtocol{
		conn:      conn,
		localPort: localPort,
		tunnelID:  tunnelID,
	}
}

// HandleTunnelMessage processes messages received from the server
func (atp *AgentTunnelProtocol) HandleTunnelMessage(messageBytes []byte) error {
	var message TunnelMessage
	if err := json.Unmarshal(messageBytes, &message); err != nil {
		return fmt.Errorf("failed to unmarshal tunnel message: %w", err)
	}

	switch message.Type {
	case "http_request":
		return atp.handleHTTPRequest(&message)
	case "websocket_upgrade":
		return atp.handleWebSocketUpgrade(&message)
	case "websocket_data":
		return atp.handleWebSocketData(&message)
	case "ping":
		return atp.handlePing(&message)
	case "pong":
		// Server acknowledged our ping - connection is alive (silent)
		return nil
	case "terminate":
		logger.Warning("Tunnel terminated by server: %s", message.ID)
		// Send close frame for graceful shutdown
		closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server initiated shutdown")
		err := atp.conn.WriteControl(
			websocket.CloseMessage,
			closeMessage,
			time.Now().Add(time.Second),
		)
		if err != nil {
			logger.Warning("Failed to send close frame: %v", err)
		}
		// Give server time to acknowledge, then close
		time.Sleep(100 * time.Millisecond)
		atp.conn.Close()
		return nil
	case "connected":
		// Tunnel connection confirmed by server (silent)
		return nil
	default:
		logger.Debug("Unknown tunnel message type: %s", message.Type)
	}

	return nil
}

func (atp *AgentTunnelProtocol) handleHTTPRequest(message *TunnelMessage) error {
	// Create HTTP request to local service
	targetURL := fmt.Sprintf("http://localhost:%d%s", atp.localPort, message.URL)

	req, err := http.NewRequest(message.Method, targetURL, bytes.NewReader(message.Body))
	if err != nil {
		return atp.sendErrorResponse(message.ID, fmt.Sprintf("Failed to create request: %v", err))
	}

	// Set headers
	for name, value := range message.Headers {
		req.Header.Set(name, value)
	}

	// Make request to local service
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return atp.sendErrorResponse(message.ID, fmt.Sprintf("Failed to connect to local service: %v", err))
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return atp.sendErrorResponse(message.ID, fmt.Sprintf("Failed to read response: %v", err))
	}

	// Convert response headers
	headers := make(map[string]string)
	for name, values := range resp.Header {
		headers[name] = strings.Join(values, ", ")
	}

	// Send response back through tunnel
	response := &TunnelMessage{
		Type:      "http_response",
		ID:        message.ID,
		Status:    resp.StatusCode,
		Headers:   headers,
		Body:      body,
		Timestamp: time.Now().Unix(),
	}

	return atp.sendMessage(response)
}

func (atp *AgentTunnelProtocol) handleWebSocketUpgrade(message *TunnelMessage) error {
	// Create WebSocket connection to local service
	localURL := fmt.Sprintf("ws://localhost:%d%s", atp.localPort, message.URL)

	// Convert headers for WebSocket dial
	header := http.Header{}
	for name, value := range message.Headers {
		header.Set(name, value)
	}

	// Connect to local WebSocket service
	localConn, resp, err := websocket.DefaultDialer.Dial(localURL, header)
	if err != nil {
		logger.Debug("Failed to connect to local WebSocket at %s: %v", localURL, err)
		// Send upgrade failure response
		response := &TunnelMessage{
			Type:      "websocket_upgrade_response",
			ID:        message.ID,
			Status:    http.StatusBadGateway,
			Error:     fmt.Sprintf("Failed to connect to local WebSocket: %v", err),
			Timestamp: time.Now().Unix(),
		}
		return atp.sendMessage(response)
	}
	defer localConn.Close()

	// Send successful upgrade response
	responseHeaders := make(map[string]string)
	if resp != nil {
		for name, values := range resp.Header {
			responseHeaders[name] = strings.Join(values, ", ")
		}
	}

	response := &TunnelMessage{
		Type:      "websocket_upgrade_response",
		ID:        message.ID,
		Status:    http.StatusSwitchingProtocols,
		Headers:   responseHeaders,
		Timestamp: time.Now().Unix(),
	}

	if err := atp.sendMessage(response); err != nil {
		return err
	}

	// Handle WebSocket data forwarding
	return atp.handleWebSocketForwarding(message.ID, localConn)
}

func (atp *AgentTunnelProtocol) handleWebSocketData(message *TunnelMessage) error {
	// This would be implemented to forward WebSocket data
	logger.Debug("Received WebSocket data for %s: %d bytes", message.ID, len(message.Body))
	return nil
}

func (atp *AgentTunnelProtocol) handleWebSocketForwarding(requestID string, localConn *websocket.Conn) error {
	// Forward messages between tunnel and local WebSocket
	done := make(chan struct{})

	// Forward from local to tunnel
	go func() {
		defer close(done)
		for {
			messageType, data, err := localConn.ReadMessage()
			if err != nil {
				logger.Debug("Local WebSocket read error: %v", err)
				return
			}

			tunnelMsg := &TunnelMessage{
				Type:      "websocket_data",
				ID:        requestID,
				Body:      data,
				Headers:   map[string]string{"message_type": strconv.Itoa(messageType)},
				Timestamp: time.Now().Unix(),
			}

			if err := atp.sendMessage(tunnelMsg); err != nil {
				logger.Debug("Failed to forward WebSocket message to tunnel: %v", err)
				return
			}
		}
	}()

	// Wait for either side to close
	<-done
	return nil
}

func (atp *AgentTunnelProtocol) handlePing(message *TunnelMessage) error {
	// Respond with pong
	pongMessage := &TunnelMessage{
		Type:      "pong",
		ID:        message.ID,
		Timestamp: time.Now().Unix(),
	}
	return atp.sendMessage(pongMessage)
}

func (atp *AgentTunnelProtocol) sendErrorResponse(requestID, errorMsg string) error {
	response := &TunnelMessage{
		Type:      "http_response",
		ID:        requestID,
		Status:    http.StatusBadGateway,
		Headers:   map[string]string{"Content-Type": "text/plain"},
		Body:      []byte(errorMsg),
		Error:     errorMsg,
		Timestamp: time.Now().Unix(),
	}
	return atp.sendMessage(response)
}

func (atp *AgentTunnelProtocol) sendMessage(message *TunnelMessage) error {
	atp.writeMutex.Lock()
	defer atp.writeMutex.Unlock()

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Set write deadline to prevent hanging on dead connections
	if err := atp.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	return atp.conn.WriteMessage(websocket.TextMessage, data)
}

// SendPing sends a ping message to the server (JSON-based, deprecated)
// Note: This is kept for backward compatibility, but WebSocket control frame pings
// (sent via WriteControl in manager.go) are now used for heartbeat instead
func (atp *AgentTunnelProtocol) SendPing() error {
	pingMessage := &TunnelMessage{
		Type:      "ping",
		ID:        fmt.Sprintf("%s-ping-%d", atp.tunnelID, time.Now().Unix()),
		Timestamp: time.Now().Unix(),
	}
	return atp.sendMessage(pingMessage)
}

// Close closes the tunnel protocol connection
func (atp *AgentTunnelProtocol) Close() error {
	if atp.conn != nil {
		return atp.conn.Close()
	}
	return nil
}
