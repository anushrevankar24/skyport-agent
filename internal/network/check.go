package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"skyport-agent/internal/config"
	"time"
)

// CheckConnectivity verifies if the agent can reach the SkyPort server
func CheckConnectivity(cfg *config.Config) error {
	// First check basic internet connectivity
	if err := checkInternetConnection(); err != nil {
		return fmt.Errorf("no internet connection")
	}

	// Then check if we can reach the SkyPort server
	if err := checkServerReachability(cfg.ServerURL); err != nil {
		return fmt.Errorf("SkyPort server is not reachable")
	}

	return nil
}

// checkInternetConnection does a quick DNS lookup to verify internet connectivity
func checkInternetConnection() error {
	// Try to resolve a well-known domain
	timeout := 3 * time.Second

	// Use custom resolver with timeout
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: timeout,
			}
			return d.Dial(network, address)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Try multiple reliable DNS targets
	targets := []string{"google.com", "github.com", "1.1.1.1"}

	for _, target := range targets {
		_, err := resolver.LookupHost(ctx, target)
		if err == nil {
			return nil // Successfully resolved, internet is working
		}
	}

	return fmt.Errorf("unable to resolve DNS - check your internet connection")
}

// checkServerReachability verifies the SkyPort server is accessible
func checkServerReachability(serverURL string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try to reach the server (any endpoint, we just want to know it's up)
	resp, err := client.Get(serverURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Server is reachable (any response is fine, even 404)
	return nil
}
