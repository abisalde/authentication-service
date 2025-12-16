package config

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	ErrSecretNotFound      = errors.New("secret not found")
	ErrInvalidSecretFormat = errors.New("invalid secret format")
)

// SecretProvider defines the interface for retrieving secrets
type SecretProvider interface {
	GetSecret(ctx context.Context, key string) (string, error)
	GetPublicKey(ctx context.Context) (*rsa.PublicKey, error)
}

// EnvironmentSecretProvider reads secrets from environment variables
// This is the simplest but least secure method - suitable for development only
type EnvironmentSecretProvider struct{}

func (e *EnvironmentSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
	}
	return value, nil
}

func (e *EnvironmentSecretProvider) GetPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	// For development, we can use symmetric keys (HS256)
	// In production, use RS256 with public/private key pairs
	return nil, errors.New("public key not supported in environment provider - use shared secret")
}

// PublicKeyProvider fetches public keys from the authentication service
// This allows microservices to validate JWT tokens without knowing the private key
type PublicKeyProvider struct {
	authServiceURL string
	publicKey      *rsa.PublicKey
	lastFetch      time.Time
	cacheDuration  time.Duration
	mu             sync.RWMutex
	httpClient     *http.Client
}

// NewPublicKeyProvider creates a new public key provider
func NewPublicKeyProvider(authServiceURL string) *PublicKeyProvider {
	return &PublicKeyProvider{
		authServiceURL: authServiceURL,
		cacheDuration:  1 * time.Hour, // Refresh public key every hour
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *PublicKeyProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Public key provider doesn't support secrets
	return "", errors.New("public key provider does not support secrets")
}

func (p *PublicKeyProvider) GetPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	p.mu.RLock()
	if p.publicKey != nil && time.Since(p.lastFetch) < p.cacheDuration {
		key := p.publicKey
		p.mu.RUnlock()
		return key, nil
	}
	p.mu.RUnlock()

	// Fetch public key from auth service
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if p.publicKey != nil && time.Since(p.lastFetch) < p.cacheDuration {
		return p.publicKey, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", p.authServiceURL+"/api/public-key", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch public key: status %d", resp.StatusCode)
	}

	pemData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	publicKey, err := parsePublicKey(pemData)
	if err != nil {
		return nil, err
	}

	p.publicKey = publicKey
	p.lastFetch = time.Now()

	return publicKey, nil
}

// FileSecretProvider reads secrets from files (e.g., Kubernetes secrets)
type FileSecretProvider struct {
	secretsDir string
}

// NewFileSecretProvider creates a new file-based secret provider
func NewFileSecretProvider(secretsDir string) *FileSecretProvider {
	return &FileSecretProvider{
		secretsDir: secretsDir,
	}
}

func (f *FileSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	path := fmt.Sprintf("%s/%s", f.secretsDir, key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
		}
		return "", fmt.Errorf("failed to read secret: %w", err)
	}
	return string(data), nil
}

func (f *FileSecretProvider) GetPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	pemData, err := f.GetSecret(ctx, "jwt_public_key.pem")
	if err != nil {
		return nil, err
	}
	return parsePublicKey([]byte(pemData))
}

// CachedSecretProvider wraps another provider with caching
type CachedSecretProvider struct {
	provider      SecretProvider
	cache         sync.Map
	cacheDuration time.Duration
}

type cachedSecret struct {
	value     string
	fetchedAt time.Time
}

// NewCachedSecretProvider creates a caching wrapper around any secret provider
func NewCachedSecretProvider(provider SecretProvider, cacheDuration time.Duration) *CachedSecretProvider {
	return &CachedSecretProvider{
		provider:      provider,
		cacheDuration: cacheDuration,
	}
}

func (c *CachedSecretProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Check cache
	if cached, ok := c.cache.Load(key); ok {
		cs := cached.(cachedSecret)
		if time.Since(cs.fetchedAt) < c.cacheDuration {
			return cs.value, nil
		}
	}

	// Fetch from provider
	value, err := c.provider.GetSecret(ctx, key)
	if err != nil {
		return "", err
	}

	// Cache the value
	c.cache.Store(key, cachedSecret{
		value:     value,
		fetchedAt: time.Now(),
	})

	return value, nil
}

func (c *CachedSecretProvider) GetPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	return c.provider.GetPublicKey(ctx)
}

// ConfigManager manages configuration and secrets for microservices
type ConfigManager struct {
	secretProvider SecretProvider
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(provider SecretProvider) *ConfigManager {
	return &ConfigManager{
		secretProvider: provider,
	}
}

// GetJWTSecret retrieves the JWT secret (for backward compatibility with HS256)
func (c *ConfigManager) GetJWTSecret(ctx context.Context) (string, error) {
	return c.secretProvider.GetSecret(ctx, "JWT_SECRET")
}

// GetJWTPublicKey retrieves the JWT public key (for RS256)
func (c *ConfigManager) GetJWTPublicKey(ctx context.Context) (*rsa.PublicKey, error) {
	return c.secretProvider.GetPublicKey(ctx)
}

// GetRedisConfig retrieves Redis configuration
func (c *ConfigManager) GetRedisConfig(ctx context.Context) (addr, password string, err error) {
	addr, err = c.secretProvider.GetSecret(ctx, "REDIS_ADDR")
	if err != nil {
		return "", "", fmt.Errorf("failed to get Redis address: %w", err)
	}

	// Password is optional
	password, _ = c.secretProvider.GetSecret(ctx, "REDIS_PASSWORD")

	return addr, password, nil
}

// parsePublicKey parses a PEM-encoded RSA public key
func parsePublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("%w: not a valid PEM block", ErrInvalidSecretFormat)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: not an RSA public key", ErrInvalidSecretFormat)
	}

	return rsaPub, nil
}

// DefaultSecretProvider returns the appropriate secret provider based on environment
func DefaultSecretProvider() SecretProvider {
	// Check if running in Kubernetes
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
		log.Println("Detected Kubernetes environment, using file-based secrets")
		return NewFileSecretProvider("/var/run/secrets/app")
	}

	// Check if public key provider should be used
	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL != "" {
		log.Printf("Using public key provider with auth service: %s", authServiceURL)
		return NewCachedSecretProvider(NewPublicKeyProvider(authServiceURL), 1*time.Hour)
	}

	// Fall back to environment variables
	log.Println("Using environment variables for secrets (development mode)")
	return &EnvironmentSecretProvider{}
}
