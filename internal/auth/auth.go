package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"skyport-agent/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/browser"
	"github.com/zalando/go-keyring"
)

const (
	KeyringService = "skyport-agent"
	KeyringUser    = "default"
)

type AuthManager struct {
	config           *config.Config
	lastTokenCheck   int64  // Unix timestamp of last validation
	lastTokenValid   bool   // Result of last validation
	lastCheckedToken string // The token that was last checked
}

type AgentAuthRequest struct {
	Token string `json:"token"`
}

type AgentAuthResponse struct {
	Valid bool `json:"valid"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
}

// ServerTunnel represents tunnel data from server API (matches server models.Tunnel)
type ServerTunnel struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	LocalPort int    `json:"local_port"`
	AuthToken string `json:"auth_token"`
	IsActive  bool   `json:"is_active"`
}

type TunnelsResponse struct {
	Tunnels []ServerTunnel `json:"tunnels"`
}

func NewAuthManager(cfg *config.Config) *AuthManager {
	return &AuthManager{config: cfg}
}

func (a *AuthManager) GetWebURL() string {
	return a.config.WebURL
}

func (a *AuthManager) StartWebAuth(callbackURL string) error {
	// Open browser to dedicated agent login page (proper OAuth flow)
	authURL := fmt.Sprintf("%s/agent-login?callback=%s", a.config.WebURL, url.QueryEscape(callbackURL))
	return browser.OpenURL(authURL)
}

// IsTokenExpired checks if a JWT token is expired locally without server validation
func (a *AuthManager) IsTokenExpired(token string) bool {
	// Parse token without verification to check expiration
	parsedToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		// If we can't parse the token, consider it expired
		return true
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return true
	}

	// Check token type - service/agent tokens never expire
	if tokenType, ok := claims["type"].(string); ok {
		if tokenType == "agent" || tokenType == "service" {
			// Service tokens have no expiration
			return false
		}
	}

	// Check expiration claim for access tokens
	if exp, ok := claims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		return time.Now().After(expTime)
	}

	// If no expiration claim and not a service token, consider it expired for safety
	return true
}

func (a *AuthManager) ValidateToken(token string) (*config.UserData, error) {
	// Validate token with backend
	reqBody := AgentAuthRequest{Token: token}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/auth/agent-auth", a.config.ServerURL),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token validation failed with status: %d", resp.StatusCode)
	}

	var authResp AgentAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !authResp.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	userData := &config.UserData{
		ID:    authResp.User.ID,
		Email: authResp.User.Email,
		Name:  authResp.User.Name,
		Token: token,
	}

	return userData, nil
}

func (a *AuthManager) SaveCredentials(userData *config.UserData) error {
	// Save token to keyring
	if err := keyring.Set(KeyringService, KeyringUser, userData.Token); err != nil {
		return fmt.Errorf("failed to save token to keyring: %w", err)
	}

	// Save user data to config file
	if err := config.SaveUserData(userData); err != nil {
		return fmt.Errorf("failed to save user data: %w", err)
	}

	return nil
}

func (a *AuthManager) LoadCredentials() (*config.UserData, error) {
	// Load user data from config file
	userData, err := config.LoadUserData()
	if err != nil {
		return nil, err
	}

	// Load token from keyring
	token, err := keyring.Get(KeyringService, KeyringUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get token from keyring: %w", err)
	}

	userData.Token = token

	// First check if token is expired locally (without server call)
	if a.IsTokenExpired(token) {
		// Token is expired, clear stored credentials
		a.ClearCredentials()
		return nil, fmt.Errorf("stored token is expired")
	}

	// Always validate with server - no offline mode
	// If server is down, user can't use tunnels anyway
	validatedUserData, err := a.ValidateToken(token)
	if err != nil {
		// Any validation error (network, server down, invalid token, etc.)
		// Clear credentials so user knows they need to re-authenticate
		a.ClearCredentials()
		return nil, fmt.Errorf("failed to validate credentials with server: %w", err)
	}

	return validatedUserData, nil
}

func (a *AuthManager) ClearCredentials() error {
	// Clear token from keyring
	keyring.Delete(KeyringService, KeyringUser)

	// Clear user data from config file
	config.ClearUserData()

	// Clear cache
	a.lastTokenCheck = 0
	a.lastTokenValid = false
	a.lastCheckedToken = ""

	return nil
}

func (a *AuthManager) LoginWithToken(token string) (*config.UserData, error) {
	// Validate the token
	userData, err := a.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	// Save credentials
	if err := a.SaveCredentials(userData); err != nil {
		return nil, err
	}

	return userData, nil
}

func (a *AuthManager) ParseAuthURL(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Check if this is a skyport auth URL
	if parsedURL.Scheme != "skyport" || parsedURL.Host != "auth" {
		return "", fmt.Errorf("invalid auth URL")
	}

	// Extract token from query parameters
	query := parsedURL.Query()
	token := query.Get("token")
	if token == "" {
		return "", fmt.Errorf("no token found in URL")
	}

	return token, nil
}

func (a *AuthManager) FetchTunnels(token string) ([]config.Tunnel, error) {
	// Create HTTP client
	client := &http.Client{}

	// Create request
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/tunnels", a.config.ServerURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/json")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tunnels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tunnels with status: %d", resp.StatusCode)
	}

	// Parse response
	var tunnelsResp TunnelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tunnelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode tunnels response: %w", err)
	}

	// Convert server tunnels to agent config tunnels
	var configTunnels []config.Tunnel
	for _, serverTunnel := range tunnelsResp.Tunnels {
		configTunnel := config.Tunnel{
			ID:        serverTunnel.ID,
			Name:      serverTunnel.Name,
			Subdomain: serverTunnel.Subdomain,
			LocalPort: serverTunnel.LocalPort,
			AuthToken: serverTunnel.AuthToken,
			IsActive:  serverTunnel.IsActive,
			AutoStart: false, // Default to false, can be set by user
		}
		configTunnels = append(configTunnels, configTunnel)
	}

	return configTunnels, nil
}

// GetStoredToken retrieves the stored authentication token
func (am *AuthManager) GetStoredToken() (string, error) {
	token, err := keyring.Get(KeyringService, KeyringUser)
	if err != nil {
		return "", fmt.Errorf("failed to get token from keyring: %w", err)
	}
	return token, nil
}

// IsAuthenticated checks if the user is currently authenticated with a valid token
// This always requires server validation - no offline mode
func (am *AuthManager) IsAuthenticated() bool {
	// Try to load complete credentials (includes server validation)
	userData, err := am.LoadCredentials()
	if err != nil {
		return false
	}

	// If we successfully loaded and validated credentials, user is authenticated
	return userData != nil && userData.Token != ""
}

// GetValidToken returns a valid token, refreshing if necessary
func (am *AuthManager) GetValidToken() (string, error) {
	token, err := am.GetStoredToken()
	if err != nil {
		return "", fmt.Errorf("no stored token: %w", err)
	}

	// For now, return the stored token
	// In a full implementation, you would validate the token and refresh if needed
	return token, nil
}

// OpenURL opens a URL in the default browser
func (am *AuthManager) OpenURL(url string) error {
	return browser.OpenURL(url)
}
