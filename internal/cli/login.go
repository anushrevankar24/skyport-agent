package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"skyport-agent/internal/auth"
	"skyport-agent/internal/config"
	"time"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with SkyPort",
	Long: `Login to your SkyPort account using your email and password.

Example:
  skyport login`,
	Run: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from SkyPort",
	Long: `Logout from your SkyPort account and clear all stored credentials.

Example:
  skyport logout`,
	Run: runLogout,
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
}

func runLogin(cmd *cobra.Command, args []string) {
	fmt.Println("Starting SkyPort login...")

	// Managers
	configManager := config.NewConfigManager()
	defaultConfig := config.Load()
	authManager := auth.NewAuthManager(defaultConfig)

	// Check if already logged in
	// Note: We always validate with server - no offline mode
	// If server is down, user can't use tunnels anyway
	if authManager.IsAuthenticated() {
		userData, err := authManager.LoadCredentials()
		if err == nil {
			fmt.Printf("Already logged in as %s!\n", userData.Name)
			fmt.Println("Use 'skyport tunnel list' to see your tunnels")
			return
		}
		// If LoadCredentials failed (server down, token invalid, etc.), continue to login
		fmt.Println("Session validation failed. Please log in again...")
	}

	// Start local callback server
	urlHandler := auth.NewURLHandler(authManager)
	callbackURL, err := urlHandler.StartServer()
	if err != nil {
		log.Fatalf("Failed to start local callback server: %v", err)
	}

	// Open browser to login page with callback
	if err := authManager.StartWebAuth(callbackURL); err != nil {
		_ = urlHandler.Stop()
		log.Fatalf("Failed to open browser for login: %v", err)
	}

	// Wait for token (5 minutes)
	token, err := urlHandler.WaitForToken(5 * time.Minute)
	_ = urlHandler.Stop()
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}

	// Validate and persist via auth manager (keyring + user.json)
	userData, err := authManager.LoginWithToken(token)
	if err != nil {
		log.Fatalf("Failed to process authentication token: %v", err)
	}

	// Also store token in app config for backward compatibility
	appConfig, _ := configManager.LoadConfig()
	if appConfig == nil {
		appConfig = &config.AppConfig{Tunnels: make(map[string]*config.Tunnel)}
	}
	appConfig.UserToken = token
	if err := configManager.SaveConfig(appConfig); err != nil {
		log.Printf("Warning: Failed to save token in app config: %v", err)
	}

	fmt.Printf("Login successful! Welcome, %s\n", userData.Name)
	fmt.Println("You can now use 'skyport tunnel list' to see your tunnels")
}

func runLogout(cmd *cobra.Command, args []string) {
	// Managers
	configManager := config.NewConfigManager()
	defaultConfig := config.Load()
	authManager := auth.NewAuthManager(defaultConfig)

	// Check if logged in
	userData, err := authManager.LoadCredentials()
	if err != nil {
		fmt.Println("You are not currently logged in.")
		return
	}

	userName := userData.Name
	userEmail := userData.Email

	// Clear credentials from keyring and user.json
	if err := authManager.ClearCredentials(); err != nil {
		log.Printf("Warning: Failed to clear some credentials: %v", err)
	}

	// Clear token from app config
	appConfig, err := configManager.LoadConfig()
	if err == nil && appConfig != nil {
		appConfig.UserToken = ""
		if err := configManager.SaveConfig(appConfig); err != nil {
			log.Printf("Warning: Failed to clear token from app config: %v", err)
		}
	}

	fmt.Printf("Logged out successfully!\n")
	fmt.Printf("  Account: %s (%s)\n", userName, userEmail)
	fmt.Println("\nYou can log in again using 'skyport login'")
}

func performLogin(serverURL, email, password string) (string, *config.UserData, error) {
	loginReq := LoginRequest{
		Email:    email,
		Password: password,
	}

	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal login request: %w", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/auth/login", serverURL),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("login failed: invalid credentials or server error")
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse response: %w", err)
	}

	userData := &config.UserData{
		ID:    loginResp.User.ID,
		Email: loginResp.User.Email,
		Name:  loginResp.User.Name,
	}

	return loginResp.Token, userData, nil
}
