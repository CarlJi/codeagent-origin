package github

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v58/github"
	"github.com/qiniu/codeagent/pkg/models"
	"github.com/qiniu/x/log"
)

// GitHubTokenManager manages GitHub App access tokens with synchronous refresh
type GitHubTokenManager interface {
	// GetAccessToken gets access token for an organization, refreshing if needed
	GetAccessToken(ctx context.Context, org string) (string, error)
	// InvalidateToken invalidates cached token for an organization
	InvalidateToken(org string)
	// Close cleans up resources
	Close()
}

// tokenCacheEntry represents a cached token entry
type tokenCacheEntry struct {
	Token          string
	ExpiresAt      time.Time
	InstallationID int64
	mutex          sync.RWMutex
}

// githubTokenManager implements GitHubTokenManager
type githubTokenManager struct {
	clientManager ClientManagerInterface
	cache         map[string]*tokenCacheEntry // org -> token entry
	cacheMutex    sync.RWMutex
}

// NewGitHubTokenManager creates a new GitHub token manager
func NewGitHubTokenManager(clientManager ClientManagerInterface) GitHubTokenManager {
	return &githubTokenManager{
		clientManager: clientManager,
		cache:         make(map[string]*tokenCacheEntry),
		cacheMutex:    sync.RWMutex{},
	}
}

// GetAccessToken gets access token for an organization, automatically refreshing if expired
func (tm *githubTokenManager) GetAccessToken(ctx context.Context, org string) (string, error) {
	// Try to get from cache first
	tm.cacheMutex.RLock()
	entry, exists := tm.cache[org]
	tm.cacheMutex.RUnlock()

	if exists {
		entry.mutex.RLock()
		token := entry.Token
		expiresAt := entry.ExpiresAt
		entry.mutex.RUnlock()

		// Check if token is still valid (with 5 minute buffer)
		if time.Now().Add(5 * time.Minute).Before(expiresAt) {
			log.Infof("Using cached access token for org: %s (expires in %.1f minutes)",
				org, time.Until(expiresAt).Minutes())
			return token, nil
		}

		log.Infof("Cached token for org %s expired or expiring soon, refreshing", org)
	}

	// Token doesn't exist or is expired, refresh it
	return tm.refreshTokenForOrg(ctx, org)
}

// InvalidateToken invalidates cached token for an organization
func (tm *githubTokenManager) InvalidateToken(org string) {
	tm.cacheMutex.Lock()
	defer tm.cacheMutex.Unlock()

	if entry, exists := tm.cache[org]; exists {
		entry.mutex.Lock()
		entry.Token = ""
		entry.ExpiresAt = time.Time{}
		entry.mutex.Unlock()
		log.Infof("Invalidated cached token for org: %s", org)
	}
}

// Close cleans up resources
func (tm *githubTokenManager) Close() {
	tm.cacheMutex.Lock()
	defer tm.cacheMutex.Unlock()
	tm.cache = make(map[string]*tokenCacheEntry)
}

// refreshTokenForOrg refreshes the access token for a specific organization
func (tm *githubTokenManager) refreshTokenForOrg(ctx context.Context, org string) (string, error) {
	log.Infof("Refreshing access token for org: %s", org)

	// Create a dummy repository to get client for this org
	repo := &models.Repository{Owner: org, Name: "dummy"}
	client, err := tm.clientManager.GetClient(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get client for org %s: %w", org, err)
	}

	// Extract token and expiration from the GitHub client
	token, expiresAt, installationID, err := tm.extractTokenFromClient(client.client)
	if err != nil {
		return "", fmt.Errorf("failed to extract token from client: %w", err)
	}

	// Update cache with new token
	tm.updateTokenCache(org, token, expiresAt, installationID)

	log.Infof("Successfully refreshed access token for org: %s (expires at: %s)",
		org, expiresAt.Format(time.RFC3339))
	return token, nil
}

// extractTokenFromClient extracts access token from GitHub client using reflection
func (tm *githubTokenManager) extractTokenFromClient(client *github.Client) (string, time.Time, int64, error) {
	// Get the HTTP client from GitHub client
	httpClient := client.Client()
	if httpClient == nil {
		return "", time.Time{}, 0, fmt.Errorf("no HTTP client found")
	}

	// Check if transport is ghinstallation.Transport
	transport := httpClient.Transport
	if transport == nil {
		return "", time.Time{}, 0, fmt.Errorf("no transport found")
	}

	// Use reflection to access private fields in ghinstallation.Transport
	transportValue := reflect.ValueOf(transport)
	if transportValue.Kind() == reflect.Ptr {
		transportValue = transportValue.Elem()
	}

	// Look for installation transport
	if transportValue.Type().String() == "ghinstallation.Transport" {
		return tm.extractFromInstallationTransport(transportValue)
	}

	return "", time.Time{}, 0, fmt.Errorf("unsupported transport type: %s", transportValue.Type().String())
}

// extractFromInstallationTransport extracts token from ghinstallation.Transport using reflection
func (tm *githubTokenManager) extractFromInstallationTransport(transportValue reflect.Value) (string, time.Time, int64, error) {
	// Try to get installation ID first
	installationIDField := transportValue.FieldByName("installationID")
	if !installationIDField.IsValid() {
		return "", time.Time{}, 0, fmt.Errorf("installationID field not found")
	}

	installationID := installationIDField.Int()

	// Look for token cache fields (ghinstallation might cache tokens internally)
	// Try to get cached token fields
	tokenField := transportValue.FieldByName("token")
	expiryField := transportValue.FieldByName("tokenExpiry")

	if tokenField.IsValid() && expiryField.IsValid() && tokenField.Kind() == reflect.String {
		token := tokenField.String()
		if token != "" {
			// Try to get expiry time
			if expiryField.Type() == reflect.TypeOf(time.Time{}) {
				expiryTime := expiryField.Interface().(time.Time)
				return token, expiryTime, installationID, nil
			}
		}
	}

	// If we can't extract from cache, make a simple API call to force token generation
	// and then try to intercept it
	return tm.generateTokenViaAPICall(transportValue, installationID)
}

// generateTokenViaAPICall generates a token by making an API call and intercepting the Authorization header
func (tm *githubTokenManager) generateTokenViaAPICall(transportValue reflect.Value, installationID int64) (string, time.Time, int64, error) {
	// Create a custom round tripper to intercept the Authorization header
	interceptor := &tokenInterceptor{}

	// Get the original round tripper
	rtField := transportValue.FieldByName("tr")
	if !rtField.IsValid() {
		// Try different field names
		rtField = transportValue.FieldByName("Transport")
		if !rtField.IsValid() {
			rtField = transportValue.FieldByName("RoundTripper")
		}
	}

	var originalRT http.RoundTripper
	if rtField.IsValid() && rtField.Interface() != nil {
		if rt, ok := rtField.Interface().(http.RoundTripper); ok {
			originalRT = rt
		}
	}

	if originalRT == nil {
		originalRT = http.DefaultTransport
	}

	interceptor.RoundTripper = originalRT

	// Create a temporary HTTP client with our interceptor
	tempClient := &http.Client{Transport: interceptor}

	// Make a simple API call to trigger token generation
	req, err := http.NewRequest("GET", "https://api.github.com/installation/repositories", nil)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Use the transport's RoundTrip method directly
	transportInterface := transportValue.Addr().Interface()
	if rt, ok := transportInterface.(http.RoundTripper); ok {
		resp, err := rt.RoundTrip(req)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}

	// Check if we intercepted a token
	if interceptor.lastToken != "" {
		// Parse JWT to get expiration time
		expiresAt := tm.parseJWTExpiration(interceptor.lastToken)
		return interceptor.lastToken, expiresAt, installationID, nil
	}

	// If interception failed, try using the temp client
	resp, err := tempClient.Do(req)
	if err == nil && resp != nil {
		resp.Body.Close()
		if interceptor.lastToken != "" {
			expiresAt := tm.parseJWTExpiration(interceptor.lastToken)
			return interceptor.lastToken, expiresAt, installationID, nil
		}
	}

	return "", time.Time{}, 0, fmt.Errorf("failed to extract or generate access token")
}

// tokenInterceptor intercepts HTTP requests to capture Authorization tokens
type tokenInterceptor struct {
	http.RoundTripper
	lastToken string
}

func (t *tokenInterceptor) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture the Authorization header
	if auth := req.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "token ") {
			t.lastToken = strings.TrimPrefix(auth, "token ")
		} else if strings.HasPrefix(auth, "Bearer ") {
			t.lastToken = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	return t.RoundTripper.RoundTrip(req)
}

// parseJWTExpiration parses JWT token to extract expiration time
func (tm *githubTokenManager) parseJWTExpiration(token string) time.Time {
	// GitHub App tokens are typically valid for 1 hour
	// Return current time + 1 hour as a safe default
	return time.Now().Add(1 * time.Hour)
}

// updateTokenCache updates the token cache for an organization
func (tm *githubTokenManager) updateTokenCache(org, token string, expiresAt time.Time, installationID int64) {
	tm.cacheMutex.Lock()
	defer tm.cacheMutex.Unlock()

	entry, exists := tm.cache[org]
	if !exists {
		entry = &tokenCacheEntry{}
		tm.cache[org] = entry
	}

	entry.mutex.Lock()
	entry.Token = token
	entry.ExpiresAt = expiresAt
	entry.InstallationID = installationID
	entry.mutex.Unlock()

	log.Infof("Updated token cache for org %s (installation: %d, expires: %s)",
		org, installationID, expiresAt.Format(time.RFC3339))
}
