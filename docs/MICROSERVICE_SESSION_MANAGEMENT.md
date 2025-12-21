# Microservice Session Management Architecture

## Overview

This document describes the session management strategy for distributed microservices architecture, enabling zero-dependency session validation across services without requiring read/write operations to the authentication service.

## Architecture Principles

Following industry best practices from Amazon, Netflix, Spotify, and Google:

1. **Stateless Authentication** - JWT tokens contain all necessary claims
2. **Decentralized Validation** - Each microservice validates tokens independently
3. **Shared Secret/Public Key Infrastructure** - Services share validation keys
4. **Token Refresh Strategy** - Short-lived access tokens with long-lived refresh tokens
5. **Distributed Session Invalidation** - Redis pub/sub for real-time token blacklisting
6. **Horizontal Scalability** - No session affinity required

## Token Structure

### Access Token (JWT)
Short-lived token (12 hours) containing user claims:

```json
{
  "sub": "user_id",
  "jti": "token_unique_id",
  "iat": 1234567890,
  "exp": 1234567890,
  "nbf": 1234567890,
  "iss": "authentication-service",
  "type": "access"
}
```

### Refresh Token (JWT)
Long-lived token (15 days) for obtaining new access tokens:

```json
{
  "sub": "user_id",
  "jti": "token_unique_id",
  "iat": 1234567890,
  "exp": 1234567890,
  "nbf": 1234567890,
  "iss": "authentication-service",
  "type": "refresh"
}
```

## Session Management Flow

### 1. Initial Authentication
```
Client → Auth Service: Login credentials
Auth Service → Client: {accessToken, refreshToken}
Auth Service → Redis: Store refresh token hash (optional)
```

### 2. Accessing Protected Resources
```
Client → Microservice A: Request + JWT Access Token
Microservice A: Validates JWT locally (no external call)
Microservice A → Client: Response
```

### 3. Token Refresh
```
Client → Auth Service: Refresh token
Auth Service: Validates refresh token
Auth Service → Client: New {accessToken, refreshToken}
```

### 4. Logout (Session Invalidation)
```
Client → Auth Service: Logout + Access Token
Auth Service → Redis: Blacklist token (TTL = remaining token life)
Auth Service → Redis Pub/Sub: Publish invalidation event
All Microservices: Subscribe to invalidation events, cache blacklist
```

## Implementation for Other Microservices

### Option 1: Shared JWT Validation Library (Recommended)

Create a shared Go module that other microservices can import:

```go
// pkg/session/validator.go
package session

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/redis/go-redis/v9"
)

type SessionValidator struct {
    secretKey       []byte
    redisClient     *redis.Client
    blacklistCache  sync.Map
    issuer          string
}

func NewSessionValidator(jwtSecret string, redisAddr string, redisPassword string) *SessionValidator {
    return &SessionValidator{
        secretKey: []byte(jwtSecret),
        redisClient: redis.NewClient(&redis.Options{
            Addr:     redisAddr,
            Password: redisPassword,
            DB:       0,
        }),
        issuer: "authentication-service",
    }
}

func (sv *SessionValidator) ValidateAccessToken(tokenString string) (*Claims, error) {
    // 1. Check local blacklist cache
    if sv.isBlacklisted(tokenString) {
        return nil, fmt.Errorf("token is blacklisted")
    }

    // 2. Validate JWT signature and claims
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return sv.secretKey, nil
    })

    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // 3. Verify it's an access token
    if claims.Type != "access" {
        return nil, fmt.Errorf("not an access token")
    }

    // 4. Check Redis blacklist (fallback)
    ctx := context.Background()
    if sv.isBlacklistedInRedis(ctx, tokenString) {
        sv.blacklistCache.Store(tokenString, true)
        return nil, fmt.Errorf("token is blacklisted")
    }

    return claims, nil
}

func (sv *SessionValidator) isBlacklisted(token string) bool {
    _, exists := sv.blacklistCache.Load(token)
    return exists
}

func (sv *SessionValidator) isBlacklistedInRedis(ctx context.Context, token string) bool {
    key := fmt.Sprintf("blacklist:%s", token)
    val, err := sv.redisClient.Get(ctx, key).Result()
    return err == nil && val == "blacklisted"
}

func (sv *SessionValidator) SubscribeToInvalidations(ctx context.Context) {
    pubsub := sv.redisClient.Subscribe(ctx, "token_invalidation")
    defer pubsub.Close()

    ch := pubsub.Channel()
    for msg := range ch {
        // Add to local cache
        sv.blacklistCache.Store(msg.Payload, true)
        
        // Set expiration timer to remove from cache
        go func(token string) {
            time.Sleep(12 * time.Hour) // Access token expiry
            sv.blacklistCache.Delete(token)
        }(msg.Payload)
    }
}

type Claims struct {
    Type string `json:"type"`
    jwt.RegisteredClaims
}
```

### Usage in Other Microservices

```go
// In your microservice main.go
validator := session.NewSessionValidator(
    os.Getenv("JWT_SECRET"),
    os.Getenv("REDIS_ADDR"),
    os.Getenv("REDIS_PASSWORD"),
)

// Start listening for invalidation events
go validator.SubscribeToInvalidations(context.Background())

// In your HTTP middleware
func AuthMiddleware(validator *session.SessionValidator) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            
            claims, err := validator.ValidateAccessToken(tokenString)
            if err != nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            // Add claims to context
            ctx := context.WithValue(r.Context(), "userID", claims.Subject)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Option 2: JWT Public Key Infrastructure (Advanced)

For enhanced security, use asymmetric encryption:

1. **Auth Service**: Signs tokens with private key (RS256)
2. **Other Services**: Validate with public key
3. **Key Rotation**: Support multiple public keys with key IDs

```go
// Auth service signs with private key
privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
tokenString, _ := token.SignedString(privateKey)

// Other services validate with public key
publicKey := &privateKey.PublicKey
token, _ := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
    return publicKey, nil
})
```

## Distributed Session Invalidation

### Redis Pub/Sub Pattern

```go
// In authentication service - publish invalidation
func (s *AuthService) InvalidateToken(ctx context.Context, token string, ttl time.Duration) error {
    // Store in Redis
    key := fmt.Sprintf("blacklist:%s", token)
    if err := s.cache.Set(ctx, key, "blacklisted", ttl); err != nil {
        return err
    }
    
    // Publish to all subscribers
    return s.redisClient.Publish(ctx, "token_invalidation", token).Err()
}
```

### Alternative: Redis Streams for Guaranteed Delivery

For critical invalidation events, use Redis Streams with consumer groups:

```go
// Publisher (Auth Service)
s.redisClient.XAdd(ctx, &redis.XAddArgs{
    Stream: "token_invalidations",
    MaxLen: 10000,
    Values: map[string]interface{}{
        "token": token,
        "timestamp": time.Now().Unix(),
    },
})

// Consumer (Other Microservices)
s.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
    Group:    "microservice_group",
    Consumer: "instance_id",
    Streams:  []string{"token_invalidations", ">"},
    Count:    100,
    Block:    time.Second,
})
```

## Load Balancing Considerations

### 1. Stateless Design
- No sticky sessions required
- Any instance can validate any token
- Scale horizontally without session migration

### 2. Token Synchronization
- All instances subscribe to Redis pub/sub
- Local cache for performance
- Redis as source of truth

### 3. Health Checks
```go
func HealthCheck(w http.ResponseWriter, r *http.Request) {
    // Check Redis connectivity
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()
    
    if err := redisClient.Ping(ctx).Err(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    
    w.WriteHeader(http.StatusOK)
}
```

### 4. Circuit Breaker for Redis
```go
// If Redis is down, still validate JWT
// But skip blacklist check (degraded mode)
func (sv *SessionValidator) ValidateWithFallback(token string) (*Claims, error) {
    claims, err := sv.validateJWTOnly(token)
    if err != nil {
        return nil, err
    }
    
    // Try blacklist check, but don't fail if Redis is down
    if sv.isHealthy() {
        if sv.isBlacklistedInRedis(context.Background(), token) {
            return nil, fmt.Errorf("token is blacklisted")
        }
    }
    
    return claims, nil
}
```

## Scalability Patterns

### 1. Token Caching
- Cache valid tokens locally with TTL
- Reduces Redis reads by 90%+
- Invalidate cache on logout events

### 2. Multi-Region Deployment
- Redis Cluster with replication
- Pub/sub across regions
- Accept eventual consistency (< 100ms)

### 3. Rate Limiting
- Per-token rate limits
- Distributed rate limiting with Redis
- Sliding window algorithm

```go
func (sv *SessionValidator) CheckRateLimit(userID string, limit int, window time.Duration) bool {
    key := fmt.Sprintf("rate_limit:%s", userID)
    count, _ := sv.redisClient.Incr(context.Background(), key).Result()
    
    if count == 1 {
        sv.redisClient.Expire(context.Background(), key, window)
    }
    
    return count <= int64(limit)
}
```

## Security Best Practices

### 1. Token Lifetime
- **Access Token**: 12 hours (balance security vs. UX)
- **Refresh Token**: 15 days (force re-authentication)
- **Adjust based on risk**: Banking = shorter, Social = longer

### 2. Token Rotation
- Issue new refresh token on each refresh
- Invalidate old refresh token
- Detect token replay attacks

### 3. Secure Storage
- **Client-side**: HttpOnly cookies for web
- **Mobile**: Secure keychain/keystore
- **Never** in localStorage

### 4. Monitoring & Alerts
- Track failed validation attempts
- Alert on suspicious patterns
- Log all token invalidations

## Migration Strategy

### Phase 1: Enable JWT Validation in Auth Service
- ✅ Already implemented

### Phase 2: Create Shared Session Library
- Package as Go module
- Publish to private registry or use replace directives

### Phase 3: Update Microservices
- Add session validation middleware
- Subscribe to invalidation events
- Remove auth service dependencies

### Phase 4: Monitoring & Optimization
- Add distributed tracing
- Optimize cache hit rates
- Tune TTL values

## Example: Complete Microservice Setup

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "strings"
    
    "github.com/abisalde/authentication-service/pkg/session"
)

func main() {
    // Initialize session validator
    validator := session.NewSessionValidator(
        os.Getenv("JWT_SECRET"),
        os.Getenv("REDIS_ADDR"),
        os.Getenv("REDIS_PASSWORD"),
    )
    
    // Start listening for token invalidations
    ctx := context.Background()
    go validator.SubscribeToInvalidations(ctx)
    
    // Setup HTTP server with auth middleware
    mux := http.NewServeMux()
    mux.HandleFunc("/api/protected", AuthRequired(validator, protectedHandler))
    mux.HandleFunc("/health", healthHandler)
    
    log.Printf("Starting service on :8081")
    http.ListenAndServe(":8081", mux)
}

func AuthRequired(validator *session.SessionValidator, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Missing authorization header", http.StatusUnauthorized)
            return
        }
        
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := validator.ValidateAccessToken(tokenString)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        ctx := context.WithValue(r.Context(), "userID", claims.Subject)
        ctx = context.WithValue(ctx, "tokenID", claims.ID)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
}

func protectedHandler(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(string)
    w.Write([]byte("Hello user: " + userID))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("OK"))
}
```

## Performance Metrics

Expected performance with this architecture:

- **Token Validation**: < 1ms (local JWT validation)
- **Blacklist Check**: < 5ms (Redis read with local cache)
- **Cache Hit Rate**: > 95% (with 5-minute local cache)
- **Throughput**: > 10,000 requests/sec per instance
- **Latency Impact**: < 2ms added to request

## Comparison with Industry Leaders

### Netflix
- Uses JWT with short expiration
- Zuul gateway validates tokens
- Services trust gateway validation

### Amazon (AWS)
- API Gateway validates JWT
- Cognito as identity provider
- Services receive validated claims

### Spotify
- OAuth 2.0 with JWT
- Token introspection endpoint (cached)
- Distributed session management

### Google
- Public key infrastructure
- Short-lived tokens (1 hour)
- Service accounts for inter-service communication

## Conclusion

This architecture provides:
- ✅ Zero read/write to auth service for validation
- ✅ Horizontal scalability
- ✅ Load balancer friendly (no sticky sessions)
- ✅ Real-time session invalidation
- ✅ Industry-proven patterns
- ✅ High performance and low latency
- ✅ Security best practices
