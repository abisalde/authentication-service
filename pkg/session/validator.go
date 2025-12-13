package session

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

var (
	ErrTokenBlacklisted = errors.New("token is blacklisted")
	ErrInvalidToken     = errors.New("invalid token")
	ErrNotAccessToken   = errors.New("not an access token")
	ErrTokenExpired     = errors.New("token expired")
)

// Claims represents the JWT claims structure
type Claims struct {
	Type string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// SessionValidator validates JWT tokens for microservices without calling the auth service
type SessionValidator struct {
	secretKey      []byte
	redisClient    *redis.Client
	blacklistCache sync.Map
	issuer         string
	clockSkew      time.Duration
	redisHealthy   bool
	healthMux      sync.RWMutex
}

// Config holds the configuration for SessionValidator
type Config struct {
	JWTSecret     string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Issuer        string
	ClockSkew     time.Duration
}

// NewSessionValidator creates a new session validator instance
func NewSessionValidator(cfg Config) (*SessionValidator, error) {
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT secret is required")
	}
	if cfg.RedisAddr == "" {
		return nil, errors.New("Redis address is required")
	}

	if cfg.Issuer == "" {
		cfg.Issuer = "authentication-service"
	}
	if cfg.ClockSkew == 0 {
		cfg.ClockSkew = 30 * time.Second
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v. Validator will work in degraded mode.", err)
	}

	sv := &SessionValidator{
		secretKey:    []byte(cfg.JWTSecret),
		redisClient:  redisClient,
		issuer:       cfg.Issuer,
		clockSkew:    cfg.ClockSkew,
		redisHealthy: true,
	}

	// Start health check goroutine
	go sv.monitorRedisHealth()

	return sv, nil
}

// ValidateAccessToken validates a JWT access token
// It checks:
// 1. Local blacklist cache
// 2. JWT signature and claims
// 3. Token type (must be "access")
// 4. Redis blacklist (if available)
func (sv *SessionValidator) ValidateAccessToken(tokenString string) (*Claims, error) {
	// 1. Check local blacklist cache
	if sv.isBlacklisted(tokenString) {
		return nil, ErrTokenBlacklisted
	}

	// 2. Validate JWT signature and claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return sv.secretKey, nil
	}, jwt.WithLeeway(sv.clockSkew))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// 3. Verify it's an access token
	if claims.Type != "access" {
		return nil, ErrNotAccessToken
	}

	// 4. Verify issuer
	if claims.Issuer != sv.issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", sv.issuer, claims.Issuer)
	}

	// 5. Check Redis blacklist (if Redis is healthy)
	if sv.isRedisHealthy() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		if sv.isBlacklistedInRedis(ctx, tokenString) {
			// Add to local cache for faster subsequent checks
			sv.blacklistCache.Store(tokenString, true)
			return nil, ErrTokenBlacklisted
		}
	}

	return claims, nil
}

// ValidateRefreshToken validates a JWT refresh token
func (sv *SessionValidator) ValidateRefreshToken(tokenString string) (*Claims, error) {
	// Similar to ValidateAccessToken but checks for refresh token type
	if sv.isBlacklisted(tokenString) {
		return nil, ErrTokenBlacklisted
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return sv.secretKey, nil
	}, jwt.WithLeeway(sv.clockSkew))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Type != "refresh" {
		return nil, errors.New("not a refresh token")
	}

	if claims.Issuer != sv.issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", sv.issuer, claims.Issuer)
	}

	return claims, nil
}

// isBlacklisted checks if token is in local cache
func (sv *SessionValidator) isBlacklisted(token string) bool {
	_, exists := sv.blacklistCache.Load(token)
	return exists
}

// isBlacklistedInRedis checks if token is blacklisted in Redis
func (sv *SessionValidator) isBlacklistedInRedis(ctx context.Context, token string) bool {
	key := fmt.Sprintf("blacklist:%s", token)
	val, err := sv.redisClient.Get(ctx, key).Result()
	return err == nil && val == "blacklisted"
}

// SubscribeToInvalidations subscribes to token invalidation events from Redis pub/sub
// This should be run as a goroutine: go validator.SubscribeToInvalidations(ctx)
func (sv *SessionValidator) SubscribeToInvalidations(ctx context.Context) {
	for {
		err := sv.subscribeToInvalidationsInternal(ctx)
		if err != nil {
			log.Printf("Subscription error: %v. Reconnecting in 5 seconds...", err)
			time.Sleep(5 * time.Second)
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			log.Println("Stopping invalidation subscription")
			return
		default:
		}
	}
}

func (sv *SessionValidator) subscribeToInvalidationsInternal(ctx context.Context) error {
	pubsub := sv.redisClient.Subscribe(ctx, "token_invalidation")
	defer pubsub.Close()

	log.Println("Subscribed to token invalidation events")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return errors.New("channel closed")
			}
			
			token := msg.Payload
			log.Printf("Received invalidation event for token: %s...", token[:10])
			
			// Add to local cache
			sv.blacklistCache.Store(token, true)
			
			// Set expiration timer to remove from cache
			// Access tokens expire in 12 hours
			go func(tk string) {
				time.Sleep(12 * time.Hour)
				sv.blacklistCache.Delete(tk)
				log.Printf("Removed expired blacklist entry for token: %s...", tk[:10])
			}(token)
		}
	}
}

// monitorRedisHealth periodically checks Redis connectivity
func (sv *SessionValidator) monitorRedisHealth() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := sv.redisClient.Ping(ctx).Err()
		cancel()

		sv.healthMux.Lock()
		sv.redisHealthy = (err == nil)
		sv.healthMux.Unlock()

		if err != nil {
			log.Printf("Redis health check failed: %v", err)
		}
	}
}

// isRedisHealthy returns the current Redis health status
func (sv *SessionValidator) isRedisHealthy() bool {
	sv.healthMux.RLock()
	defer sv.healthMux.RUnlock()
	return sv.redisHealthy
}

// GetUserID extracts user ID from claims
func (c *Claims) GetUserID() string {
	return c.Subject
}

// GetTokenID extracts token ID from claims
func (c *Claims) GetTokenID() string {
	return c.ID
}

// IsAccessToken checks if the token is an access token
func (c *Claims) IsAccessToken() bool {
	return c.Type == "access"
}

// IsRefreshToken checks if the token is a refresh token
func (c *Claims) IsRefreshToken() bool {
	return c.Type == "refresh"
}

// Close closes the Redis connection
func (sv *SessionValidator) Close() error {
	return sv.redisClient.Close()
}

// GetRedisClient returns the underlying Redis client for advanced usage
func (sv *SessionValidator) GetRedisClient() *redis.Client {
	return sv.redisClient
}
