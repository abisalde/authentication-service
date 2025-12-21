# Session Management Example

This example demonstrates how to implement multi-device session management with the authentication service.

## Features

- 📱 **Device Tracking** - Track all active sessions per user with device info
- 🔒 **Session Revocation** - Revoke specific device sessions
- 🚪 **Logout from All Devices** - Security feature to invalidate all sessions
- 🎯 **Max Sessions Enforcement** - Limit concurrent sessions per user
- 🕐 **Session Activity Tracking** - Monitor last used time for each session

## Running the Example

### Prerequisites

- Go 1.24.3+
- Redis running on localhost:6379

### Start Redis

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### Run the Example

```bash
export REDIS_ADDR="localhost:6379"
export REDIS_PASSWORD=""  # Empty if no password

go run main.go
```

The server will start on `http://localhost:8082`

## API Endpoints

### 1. Login (Create Session)

Creates a new session for a user.

```bash
curl -X POST http://localhost:8082/login \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "token": "mock-jwt-token"
  }'
```

Response:
```json
{
  "success": true,
  "session_id": "abc123...",
  "message": "Logged in from Chrome Browser on Windows"
}
```

### 2. View All Sessions

Get all active sessions for a user.

```bash
curl "http://localhost:8082/sessions?user_id=123"
```

Response:
```json
{
  "sessions": [
    {
      "session_id": "abc123...",
      "device_type": "Desktop",
      "device_name": "Chrome Browser on Windows",
      "ip_address": "192.168.1.1",
      "created_at": "2024-01-01T10:00:00Z",
      "last_used_at": "2024-01-01T10:05:00Z",
      "is_current": true
    },
    {
      "session_id": "def456...",
      "device_type": "Mobile",
      "device_name": "iPhone",
      "ip_address": "192.168.1.2",
      "created_at": "2024-01-01T09:00:00Z",
      "last_used_at": "2024-01-01T09:30:00Z",
      "is_current": false
    }
  ],
  "count": 2
}
```

### 3. Revoke Specific Session

Logout from a specific device.

```bash
curl -X POST http://localhost:8082/sessions/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "session_id": "def456..."
  }'
```

Response:
```json
{
  "success": true,
  "message": "Session revoked successfully"
}
```

### 4. Revoke Other Sessions

Logout from all devices except the current one.

```bash
curl -X POST http://localhost:8082/sessions/revoke-others \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "current_session_id": "abc123..."
  }'
```

Response:
```json
{
  "success": true,
  "message": "All other sessions revoked successfully"
}
```

### 5. Logout from All Devices

Revoke all sessions (complete logout).

```bash
curl -X POST http://localhost:8082/logout-all \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123"
  }'
```

Response:
```json
{
  "success": true,
  "message": "Logged out from all devices"
}
```

## Testing Scenario

### Simulate Multi-Device Login

```bash
# Login from Desktop (Chrome)
curl -X POST http://localhost:8082/login \
  -H "Content-Type: application/json" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0" \
  -d '{"user_id": "123", "token": "desktop-token-123"}'

# Login from Mobile (iPhone)
curl -X POST http://localhost:8082/login \
  -H "Content-Type: application/json" \
  -H "User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148" \
  -d '{"user_id": "123", "token": "mobile-token-456"}'

# Login from Tablet (iPad)
curl -X POST http://localhost:8082/login \
  -H "Content-Type: application/json" \
  -H "User-Agent: Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15" \
  -d '{"user_id": "123", "token": "tablet-token-789"}'

# View all sessions
curl "http://localhost:8082/sessions?user_id=123"

# Revoke iPhone session
curl -X POST http://localhost:8082/sessions/revoke \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123",
    "session_id": "<session_id_from_previous_response>"
  }'
```

## Integration with Auth Service

To integrate this with your authentication service:

### 1. On Login

```go
// In your login handler
func (h *LoginHandler) EmailLogin(ctx context.Context, input model.LoginInput) (*model.LoginResponse, error) {
    // ... existing authentication logic ...
    
    // Generate tokens
    tokens, err := cookies.GenerateLoginTokenPair(user.ID)
    if err != nil {
        return nil, err
    }
    
    // Extract device info from request
    req := auth.GetHTTPRequest(ctx)
    deviceInfo := session.ExtractDeviceInfo(req)
    
    // Create session
    sessionManager := session.NewSessionManager(h.redisClient)
    sessionInfo := &session.SessionInfo{
        UserID:     strconv.FormatInt(user.ID, 10),
        DeviceType: deviceInfo.Type,
        DeviceName: deviceInfo.Name,
        IPAddress:  deviceInfo.IPAddress,
        UserAgent:  deviceInfo.UserAgent,
        TokenHash:  session.HashToken(tokens.AccessToken),
        CreatedAt:  time.Now(),
        LastUsedAt: time.Now(),
        ExpiresAt:  time.Now().Add(cookies.LoginAccessTokenExpiry),
    }
    
    // Optional: Enforce max 5 devices
    sessionManager.EnforceMaxSessions(ctx, sessionInfo.UserID, 5)
    
    if err := sessionManager.CreateSession(ctx, sessionInfo); err != nil {
        log.Printf("Failed to create session: %v", err)
        // Don't fail login if session creation fails
    }
    
    return &model.LoginResponse{
        Token:        tokens.AccessToken,
        RefreshToken: tokens.RefreshToken,
        UserID:       user.ID,
        Email:        user.Email,
    }, nil
}
```

### 2. Add GraphQL Schema

```graphql
type Session {
  sessionId: ID!
  deviceType: String!
  deviceName: String!
  ipAddress: String!
  createdAt: Time!
  lastUsedAt: Time!
  isCurrent: Boolean!
}

extend type Query {
  mySessions: [Session!]! @auth(requires: USER)
}

extend type Mutation {
  revokeSession(sessionId: ID!): Boolean! @auth(requires: USER)
  logoutOtherDevices: Boolean! @auth(requires: USER)
  logoutAllDevices: Boolean! @auth(requires: USER)
}
```

### 3. Add Resolvers

```go
func (r *queryResolver) MySessions(ctx context.Context) ([]*model.Session, error) {
    user := auth.GetCurrentUser(ctx)
    if user == nil {
        return nil, errors.Unauthenticated
    }
    
    sessionManager := session.NewSessionManager(r.redisClient)
    sessions, err := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
    if err != nil {
        return nil, err
    }
    
    // Convert to GraphQL model
    result := make([]*model.Session, len(sessions))
    for i, s := range sessions {
        result[i] = &model.Session{
            SessionID:  s.SessionID,
            DeviceType: s.DeviceType,
            DeviceName: s.DeviceName,
            IPAddress:  s.IPAddress,
            CreatedAt:  s.CreatedAt,
            LastUsedAt: s.LastUsedAt,
            // Determine if current session based on token
        }
    }
    
    return result, nil
}
```

## Security Considerations

### 1. IP Privacy
- Consider hashing IP addresses for privacy
- Or store only partial IPs (first 3 octets)

### 2. Session Limits
- Enforce reasonable max sessions (5-10)
- Auto-revoke oldest sessions when limit reached

### 3. Activity Tracking
- Update LastUsedAt on each request
- Show users when session was last active

### 4. Notifications
- Notify users of new logins
- Alert on suspicious login patterns

### 5. Device Fingerprinting
- For enhanced security, bind tokens to device fingerprints
- See `session.GenerateFingerprint()` for implementation

## Performance Tips

1. **Batch Operations**: Use Redis pipelining for bulk session operations
2. **Caching**: Cache session data in application memory with short TTL
3. **Cleanup**: Implement background worker to clean expired sessions
4. **Indexing**: Use Redis secondary indexes for faster lookups

## Troubleshooting

### "Session not found"
- Session may have expired
- Check TTL configuration
- Verify Redis is running

### "Too many sessions"
- Increase max sessions limit
- Implement session cleanup
- Consider session rotation

### High Redis Memory
- Reduce session data size
- Implement aggressive cleanup
- Use Redis maxmemory-policy

## Related Documentation

- [Multi-Device Session Management Guide](../../docs/MULTI_DEVICE_SESSION_MANAGEMENT.md)
- [Session Package Documentation](../../pkg/session/README.md)
- [Architecture Guide](../../docs/MICROSERVICE_SESSION_MANAGEMENT.md)

## License

See the main repository LICENSE file.
