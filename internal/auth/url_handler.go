package auth

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type URLHandler struct {
	authMgr  *AuthManager
	server   *http.Server
	listener net.Listener
	tokenCh  chan string
	errCh    chan error
}

func NewURLHandler(authMgr *AuthManager) *URLHandler {
	return &URLHandler{
		authMgr: authMgr,
		tokenCh: make(chan string, 1),
		errCh:   make(chan error, 1),
	}
}

func (h *URLHandler) StartServer() (string, error) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to create listener: %w", err)
	}

	h.listener = listener
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/auth", h.handleAuth)

	h.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in background
	go func() {
		if err := h.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			h.errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	return fmt.Sprintf("http://localhost:%d/auth", port), nil
}

func (h *URLHandler) handleAuth(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Check for success parameter
	success := query.Get("success")
	token := query.Get("token")

	if success == "true" && token != "" {
		// Send success response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>SkyPort Authentication</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #28a745; }
        .container { max-width: 500px; margin: 0 auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="success">Authentication Successful</h1>
        <p>Your SkyPort Agent has been successfully authenticated.</p>
        <p>You can now close this window and return to the agent.</p>
    </div>
</body>
</html>
		`))

		// Send token to channel
		select {
		case h.tokenCh <- token:
		default:
			// Channel full, ignore
		}
	} else {
		// Send error response
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>SkyPort Authentication</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .error { color: #dc3545; }
        .container { max-width: 500px; margin: 0 auto; }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="error">Authentication Failed</h1>
        <p>There was an error during authentication.</p>
        <p>Please try again or contact support.</p>
    </div>
</body>
</html>
		`))
	}
}

func (h *URLHandler) WaitForToken(timeout time.Duration) (string, error) {
	select {
	case token := <-h.tokenCh:
		return token, nil
	case err := <-h.errCh:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for authentication")
	}
}

func (h *URLHandler) Stop() error {
	if h.server != nil {
		return h.server.Close()
	}
	return nil
}

// Alternative method for custom protocol handling
func HandleCustomProtocol(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if this is a skyport auth URL
	if !strings.HasPrefix(rawURL, "skyport://") {
		return "", fmt.Errorf("not a skyport protocol URL")
	}

	// Extract token from query parameters
	query := parsedURL.Query()
	token := query.Get("token")
	if token == "" {
		return "", fmt.Errorf("no token found in URL")
	}

	return token, nil
}
