# Quick Start Guide: Microservice Session Management

This guide will help you get started with the distributed session management system in 5 minutes.

## Prerequisites

- Go 1.24.3+
- Redis 7.x
- Docker (optional, for running Redis)

## Step 1: Start Redis

```bash
# Using Docker
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Or use docker-compose (already configured in this repo)
cd deployments
docker-compose up -d redis
```

## Step 2: Set Environment Variables

```bash
export JWT_SECRET="your-super-secret-jwt-key-here"
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD=""  # Empty if no password
```

## Step 3: Run the Authentication Service

```bash
# From the repository root
go run ./cmd/server/main.go
```

The authentication service will start on `http://localhost:8080`

## Step 4: Run the Example Microservice

```bash
# In a new terminal, set the same environment variables
export JWT_SECRET="your-super-secret-jwt-key-here"
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD=""
export PORT="8081"

# Run the example microservice
go run ./examples/example-microservice/main.go
```

The example microservice will start on `http://localhost:8081`

## Step 5: Test the Flow

### 1. Register a User (via Auth Service)

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { register(input: {email: \"test@example.com\", password: \"Test1234!\"}) { message user { email } } }"
  }'
```

### 2. Login to Get Token

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { login(input: {email: \"test@example.com\", password: \"Test1234!\"}) { token userId email } }"
  }'
```

Save the token from the response.

### 3. Access Protected Endpoint (Microservice)

```bash
# Replace YOUR_TOKEN with the actual token from step 2
curl http://localhost:8081/api/protected \
  -H "Authorization: Bearer YOUR_TOKEN"
```

You should see:
```
Welcome authenticated user!
User ID: 123
Token ID: abc-def-ghi
```

### 4. Test Public Endpoint

```bash
# Without token
curl http://localhost:8081/api/public

# With token
curl http://localhost:8081/api/public \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 5. Logout (Invalidate Token)

```bash
curl -X POST http://localhost:8080/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "query": "mutation { logout }"
  }'
```

### 6. Try to Access Again (Should Fail)

```bash
curl http://localhost:8081/api/protected \
  -H "Authorization: Bearer YOUR_TOKEN"
```

You should see:
```
Invalid or expired token
```

🎉 **Success!** The microservice validated the token without calling the auth service and blocked it after logout!

## What Just Happened?

1. **Login**: Auth service generated a JWT token
2. **Access**: Microservice validated the token locally (< 1ms)
3. **Logout**: Auth service blacklisted the token and published to Redis
4. **Block**: Microservice received the invalidation event and blocked the token

**Zero calls to auth service for validation!**

## Adding to Your Microservice

### 1. Import the Package

```go
import "github.com/abisalde/authentication-service/pkg/session"
```

### 2. Initialize in main.go

```go
func main() {
    // Create validator
    validator, err := session.NewSessionValidator(session.Config{
        JWTSecret:     os.Getenv("JWT_SECRET"),
        RedisAddr:     os.Getenv("REDIS_ADDR"),
        RedisPassword: os.Getenv("REDIS_PASSWORD"),
    })
    if err != nil {
        log.Fatal(err)
    }
    defer validator.Close()

    // Subscribe to invalidations
    ctx := context.Background()
    go validator.SubscribeToInvalidations(ctx)

    // Your service setup...
}
```

### 3. Protect Your Endpoints

```go
// Option A: Middleware for all routes
mux := http.NewServeMux()
handler := session.HTTPMiddleware(validator)(mux)
http.ListenAndServe(":8080", handler)

// Option B: Per-route protection
http.HandleFunc("/api/protected", 
    session.RequireAuth(validator, yourHandler))
```

### 4. Access User Info in Handlers

```go
func yourHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := session.GetUserID(r.Context())
    if !ok {
        http.Error(w, "Unauthorized", 401)
        return
    }
    
    // Use userID for your business logic
    fmt.Fprintf(w, "Hello user %s", userID)
}
```

## Configuration Options

### Basic Configuration

```go
validator, err := session.NewSessionValidator(session.Config{
    JWTSecret:     "your-secret",
    RedisAddr:     "localhost:6379",
    RedisPassword: "optional",
})
```

### Advanced Configuration

```go
validator, err := session.NewSessionValidator(session.Config{
    JWTSecret:            "your-secret",
    RedisAddr:            "localhost:6379",
    RedisPassword:        "optional",
    RedisDB:              0,
    Issuer:               "authentication-service",
    ClockSkew:            30 * time.Second,
    AccessTokenCacheTTL:  12 * time.Hour,
    RefreshTokenCacheTTL: 15 * 24 * time.Hour,
})
```

## Troubleshooting

### "JWT secret is required"
- Set the `JWT_SECRET` environment variable
- Must match the auth service secret

### "Redis connection failed"
- Check Redis is running: `redis-cli ping`
- Verify `REDIS_ADDR` is correct
- Check firewall rules

### Token validation fails
- Ensure `JWT_SECRET` matches auth service
- Check token hasn't expired
- Verify token format: `Bearer <token>`

### Invalidation not working
- Check Redis pub/sub: `redis-cli PSUBSCRIBE token_invalidation`
- Ensure `SubscribeToInvalidations` is running
- Verify auth service is publishing events

## Performance Tips

1. **Use Local Cache**: Enabled by default (95%+ hit rate)
2. **Tune TTLs**: Match token expiration times
3. **Monitor Redis**: Keep latency < 5ms
4. **Connection Pooling**: Reuse validator instances
5. **Health Checks**: Monitor validator health

## Next Steps

- 📖 Read [Microservice Session Management Guide](./MICROSERVICE_SESSION_MANAGEMENT.md)
- 🏗️ Review [Deployment Architecture](./DEPLOYMENT_ARCHITECTURE.md)
- 📦 Check [Session Package Documentation](../pkg/session/README.md)
- 💡 Study [Example Microservice](../examples/example-microservice/)
- 📋 Read [Implementation Summary](./IMPLEMENTATION_SUMMARY.md)

## Production Checklist

Before deploying to production:

- [ ] Generate strong JWT_SECRET (32+ bytes)
- [ ] Enable Redis persistence (RDB/AOF)
- [ ] Set up Redis cluster for HA
- [ ] Configure load balancer (no sticky sessions)
- [ ] Enable HTTPS for all services
- [ ] Set up monitoring and alerting
- [ ] Test token invalidation flow
- [ ] Perform load testing
- [ ] Document incident response
- [ ] Set up backup and recovery

## Support

Need help? Check:
1. [FAQ in main documentation](./MICROSERVICE_SESSION_MANAGEMENT.md)
2. [Troubleshooting guide](../pkg/session/README.md#troubleshooting)
3. [Example microservice](../examples/example-microservice/README.md)

## License

See the main repository LICENSE file.
