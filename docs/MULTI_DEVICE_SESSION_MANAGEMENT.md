# Multi-Device Session Management & Security

## Problem Statement

When a user logs in from multiple devices (Desktop, Mobile App, Tablet), they create multiple active sessions with valid JWT tokens and refresh tokens. This creates security risks:

1. **Uncontrolled Sessions**: No visibility or control over active sessions
2. **Token Hijacking Risk**: If one device is compromised, the token can be used indefinitely
3. **No Session Revocation**: Cannot revoke specific device sessions
4. **Concurrent Access Risks**: No tracking of suspicious concurrent access patterns

## Recommended Solutions

### QUESTION ONE: What Do I Advise?

I recommend implementing a **Device-Aware Session Management System** with the following capabilities:

#### 1. **Session Tracking & Management** (Recommended - Most Common)
- Track all active sessions per user with device metadata
- Allow users to view and revoke sessions
- Implement "logout from all devices" functionality
- Set maximum concurrent sessions per user (optional)

#### 2. **Fingerprint-Based Token Binding** (Enhanced Security)
- Bind tokens to device fingerprints
- Validate device fingerprint on each request
- Automatically invalidate if fingerprint changes

#### 3. **Single-Session Mode** (Strictest - Banking Apps)
- Only allow one active session per user
- New login invalidates previous session
- Good for high-security applications

#### 4. **Anomaly Detection** (Advanced)
- Detect suspicious login patterns (location, device changes)
- Challenge verification for unusual activity
- Rate limiting per device

### QUESTION TWO: Implementation Guide

## Solution 1: Session Tracking & Management (Recommended)

This is the most common approach used by Google, Facebook, GitHub, etc.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    User Sessions                        │
│  user:123:sessions → Set of Session IDs                 │
└─────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
┌───────▼────────┐  ┌───────▼────────┐  ┌──────▼─────────┐
│ session:abc123 │  │ session:def456 │  │ session:ghi789 │
│                │  │                │  │                │
│ user_id: 123   │  │ user_id: 123   │  │ user_id: 123   │
│ device: Desktop│  │ device: iPhone │  │ device: Android│
│ ip: 1.2.3.4    │  │ ip: 5.6.7.8    │  │ ip: 9.10.11.12 │
│ token: jwt...  │  │ token: jwt...  │  │ token: jwt...  │
│ created: ...   │  │ created: ...   │  │ created: ...   │
│ last_used: ... │  │ last_used: ... │  │ last_used: ... │
└────────────────┘  └────────────────┘  └────────────────┘
```

### Implementation Steps

#### Step 1: Add Session Tracking to pkg/session

```go
// pkg/session/session_manager.go
package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionInfo represents an active user session
type SessionInfo struct {
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	DeviceType  string    `json:"device_type"`   // Desktop, Mobile, Tablet
	DeviceName  string    `json:"device_name"`   // "Chrome on Windows", "iPhone 13"
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	TokenHash   string    `json:"token_hash"`    // Hash of access token
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// SessionManager manages user sessions across devices
type SessionManager struct {
	redisClient *redis.Client
}

// NewSessionManager creates a new session manager
func NewSessionManager(redisClient *redis.Client) *SessionManager {
	return &SessionManager{
		redisClient: redisClient,
	}
}

// CreateSession creates a new session for a user
func (sm *SessionManager) CreateSession(ctx context.Context, session *SessionInfo) error {
	// Generate session ID if not provided
	if session.SessionID == "" {
		session.SessionID = generateSessionID(session.UserID, session.TokenHash)
	}

	// Store session info
	sessionKey := fmt.Sprintf("session:%s", session.SessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	if err := sm.redisClient.Set(ctx, sessionKey, sessionData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	// Add session to user's active sessions set
	userSessionsKey := fmt.Sprintf("user:%s:sessions", session.UserID)
	if err := sm.redisClient.SAdd(ctx, userSessionsKey, session.SessionID).Err(); err != nil {
		return fmt.Errorf("failed to add session to user set: %w", err)
	}

	// Set expiration on user sessions set (refresh on each new session)
	if err := sm.redisClient.Expire(ctx, userSessionsKey, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set expiration on user sessions: %w", err)
	}

	return nil
}

// GetUserSessions returns all active sessions for a user
func (sm *SessionManager) GetUserSessions(ctx context.Context, userID string) ([]*SessionInfo, error) {
	userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
	
	// Get all session IDs for the user
	sessionIDs, err := sm.redisClient.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	sessions := make([]*SessionInfo, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, err := sm.GetSession(ctx, sessionID)
		if err != nil {
			// Session might have expired, remove from set
			sm.redisClient.SRem(ctx, userSessionsKey, sessionID)
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	
	data, err := sm.redisClient.Get(ctx, sessionKey).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	var session SessionInfo
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// UpdateSessionActivity updates the last used time for a session
func (sm *SessionManager) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	session.LastUsedAt = time.Now()
	
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	return sm.redisClient.Set(ctx, sessionKey, sessionData, ttl).Err()
}

// RevokeSession revokes a specific session
func (sm *SessionManager) RevokeSession(ctx context.Context, userID, sessionID string) error {
	// Get session to get token hash for blacklisting
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Remove from user's sessions set
	userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
	if err := sm.redisClient.SRem(ctx, userSessionsKey, sessionID).Err(); err != nil {
		return fmt.Errorf("failed to remove session from user set: %w", err)
	}

	// Delete session data
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	if err := sm.redisClient.Del(ctx, sessionKey).Err(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// RevokeAllUserSessions revokes all sessions for a user (logout from all devices)
func (sm *SessionManager) RevokeAllUserSessions(ctx context.Context, userID string) error {
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if err := sm.RevokeSession(ctx, userID, session.SessionID); err != nil {
			// Log error but continue revoking other sessions
			fmt.Printf("Failed to revoke session %s: %v\n", session.SessionID, err)
		}
	}

	return nil
}

// RevokeOtherSessions revokes all sessions except the current one
func (sm *SessionManager) RevokeOtherSessions(ctx context.Context, userID, currentSessionID string) error {
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session.SessionID != currentSessionID {
			if err := sm.RevokeSession(ctx, userID, session.SessionID); err != nil {
				fmt.Printf("Failed to revoke session %s: %v\n", session.SessionID, err)
			}
		}
	}

	return nil
}

// GetSessionByTokenHash finds a session by token hash
func (sm *SessionManager) GetSessionByTokenHash(ctx context.Context, userID, tokenHash string) (*SessionInfo, error) {
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.TokenHash == tokenHash {
			return session, nil
		}
	}

	return nil, fmt.Errorf("session not found for token")
}

// generateSessionID generates a unique session ID
func generateSessionID(userID, tokenHash string) string {
	data := fmt.Sprintf("%s:%s:%d", userID, tokenHash, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// hashToken creates a hash of a token for storage
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
```

#### Step 2: Update SessionValidator to Track Sessions

```go
// pkg/session/validator.go - Add these methods

// ValidateAccessTokenWithSession validates token and updates session activity
func (sv *SessionValidator) ValidateAccessTokenWithSession(tokenString string, sessionManager *SessionManager) (*Claims, *SessionInfo, error) {
	// Validate token first
	claims, err := sv.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, nil, err
	}

	// Get token hash
	tokenHash := HashToken(tokenString)

	// Find session
	ctx := context.Background()
	session, err := sessionManager.GetSessionByTokenHash(ctx, claims.Subject, tokenHash)
	if err != nil {
		// Session not found - could be old token before session tracking
		return claims, nil, nil
	}

	// Update session activity
	if err := sessionManager.UpdateSessionActivity(ctx, session.SessionID); err != nil {
		// Log error but don't fail validation
		log.Printf("Failed to update session activity: %v", err)
	}

	return claims, session, nil
}
```

#### Step 3: Update Middleware to Extract Device Info

```go
// pkg/session/middleware.go - Enhanced version

import (
	"net"
	"strings"
)

// DeviceInfo extracts device information from request
type DeviceInfo struct {
	Type      string // Desktop, Mobile, Tablet
	Name      string // Browser/App name
	IPAddress string
	UserAgent string
}

// ExtractDeviceInfo extracts device information from HTTP request
func ExtractDeviceInfo(r *http.Request) *DeviceInfo {
	userAgent := r.Header.Get("User-Agent")
	
	deviceType := "Desktop"
	deviceName := "Unknown"
	
	// Simple device detection (use a library like mssola/user_agent for production)
	ua := strings.ToLower(userAgent)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
		deviceType = "Mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		deviceType = "Tablet"
	}
	
	// Extract device name from user agent
	if strings.Contains(ua, "chrome") {
		deviceName = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		deviceName = "Firefox"
	} else if strings.Contains(ua, "safari") {
		deviceName = "Safari"
	}
	
	// Get IP address
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		ip = host
	}
	
	return &DeviceInfo{
		Type:      deviceType,
		Name:      deviceName,
		IPAddress: ip,
		UserAgent: userAgent,
	}
}
```

#### Step 4: Update Authentication Service to Create Sessions

```go
// internal/auth/service/auth.go - Add these methods

func (s *AuthService) CreateSessionOnLogin(ctx context.Context, userID int64, accessToken string, deviceInfo *session.DeviceInfo) error {
	tokenHash := session.HashToken(accessToken)
	
	sessionInfo := &session.SessionInfo{
		UserID:     strconv.FormatInt(userID, 10),
		DeviceType: deviceInfo.Type,
		DeviceName: deviceInfo.Name,
		IPAddress:  deviceInfo.IPAddress,
		UserAgent:  deviceInfo.UserAgent,
		TokenHash:  tokenHash,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(cookies.LoginAccessTokenExpiry),
	}
	
	sessionManager := session.NewSessionManager(s.cache.RawClient())
	return sessionManager.CreateSession(ctx, sessionInfo)
}

func (s *AuthService) GetUserSessions(ctx context.Context, userID int64) ([]*session.SessionInfo, error) {
	sessionManager := session.NewSessionManager(s.cache.RawClient())
	return sessionManager.GetUserSessions(ctx, strconv.FormatInt(userID, 10))
}

func (s *AuthService) RevokeUserSession(ctx context.Context, userID int64, sessionID string) error {
	sessionManager := session.NewSessionManager(s.cache.RawClient())
	
	// Get session to blacklist token
	session, err := sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	
	// Revoke the session
	if err := sessionManager.RevokeSession(ctx, strconv.FormatInt(userID, 10), sessionID); err != nil {
		return err
	}
	
	// Note: We don't have the actual token here, only the hash
	// So we publish the session ID for invalidation
	// Microservices will need to check both blacklist and session validity
	return s.cache.RawClient().Publish(ctx, "session_invalidation", sessionID).Err()
}

func (s *AuthService) LogoutFromAllDevices(ctx context.Context, userID int64) error {
	sessionManager := session.NewSessionManager(s.cache.RawClient())
	return sessionManager.RevokeAllUserSessions(ctx, strconv.FormatInt(userID, 10))
}
```

#### Step 5: Add GraphQL Mutations/Queries

```graphql
# Add to your schema

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
  """
  Get all active sessions for the current user
  """
  mySessions: [Session!]! @auth(requires: USER)
}

extend type Mutation {
  """
  Revoke a specific session (logout from specific device)
  """
  revokeSession(sessionId: ID!): Boolean! @auth(requires: USER)
  
  """
  Revoke all sessions except current (logout from other devices)
  """
  logoutOtherDevices: Boolean! @auth(requires: USER)
  
  """
  Revoke all sessions (logout from all devices)
  """
  logoutAllDevices: Boolean! @auth(requires: USER)
}
```

### Usage Example

```go
// On login
deviceInfo := session.ExtractDeviceInfo(r)
tokens, _ := cookies.GenerateLoginTokenPair(userID)

// Create session
authService.CreateSessionOnLogin(ctx, userID, tokens.AccessToken, deviceInfo)

// User can view their sessions
sessions, _ := authService.GetUserSessions(ctx, userID)

// User can revoke a specific session
authService.RevokeUserSession(ctx, userID, "session-id-here")

// User can logout from all other devices
sessionManager.RevokeOtherSessions(ctx, userID, currentSessionID)

// User can logout from all devices
authService.LogoutFromAllDevices(ctx, userID)
```

## Solution 2: Token Fingerprinting (Enhanced Security)

### Implementation

```go
// pkg/session/fingerprint.go
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// DeviceFingerprint represents a unique device identifier
type DeviceFingerprint struct {
	UserAgent string
	IPAddress string
	// Add more fields for stronger fingerprinting
	AcceptLanguage string
	ScreenResolution string
}

// GenerateFingerprint creates a hash from device attributes
func GenerateFingerprint(fp *DeviceFingerprint) string {
	data := fmt.Sprintf("%s:%s:%s:%s", 
		fp.UserAgent, 
		fp.IPAddress, 
		fp.AcceptLanguage,
		fp.ScreenResolution,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Store fingerprint in Claims
type ClaimsWithFingerprint struct {
	Claims
	Fingerprint string `json:"fp"`
}
```

## Solution 3: Maximum Concurrent Sessions

```go
// pkg/session/session_manager.go - Add this method

// EnforceMaxSessions ensures user doesn't exceed max concurrent sessions
func (sm *SessionManager) EnforceMaxSessions(ctx context.Context, userID string, maxSessions int) error {
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	if len(sessions) >= maxSessions {
		// Revoke oldest sessions
		// Sort by created_at
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].CreatedAt.Before(sessions[j].CreatedAt)
		})

		// Remove oldest sessions to make room
		toRemove := len(sessions) - maxSessions + 1
		for i := 0; i < toRemove; i++ {
			sm.RevokeSession(ctx, userID, sessions[i].SessionID)
		}
	}

	return nil
}
```

## Security Best Practices

### 1. Session Security
- ✅ Store minimal data in sessions
- ✅ Use secure session IDs (cryptographic random)
- ✅ Set appropriate TTLs
- ✅ Clean up expired sessions

### 2. Token Security
- ✅ Never store actual tokens (use hashes)
- ✅ Blacklist tokens on session revocation
- ✅ Rotate tokens periodically
- ✅ Use short-lived access tokens

### 3. Device Security
- ✅ Detect suspicious device changes
- ✅ Challenge unusual login locations
- ✅ Rate limit per device
- ✅ Log all session activities

### 4. User Privacy
- ✅ Don't store full IP addresses (hash or partial)
- ✅ Anonymize user agents
- ✅ Comply with GDPR/privacy laws
- ✅ Allow users to view/control sessions

## Performance Considerations

- Use Redis Sets for efficient session lookups
- Implement session cleanup worker
- Cache session data in application memory
- Use pagination for session lists (if user has many)

## Migration Path

1. **Phase 1**: Add session tracking to new logins
2. **Phase 2**: Add UI for session management
3. **Phase 3**: Enable automatic cleanup
4. **Phase 4**: Add security features (anomaly detection)

## Comparison: Solutions

| Feature | Session Tracking | Fingerprinting | Single Session |
|---------|-----------------|----------------|----------------|
| User Control | ✅ High | ❌ No | ❌ No |
| Security | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| UX Impact | ✅ Minimal | ⚠️ Medium | ❌ High |
| Complexity | ⭐⭐ | ⭐⭐⭐ | ⭐ |
| Industry Use | Google, GitHub | Banking Apps | High-Security |

## Recommendation Summary

**For most applications**: Use **Solution 1 (Session Tracking)** - it provides the best balance of security, user experience, and flexibility.

**For high-security apps**: Combine **Solution 1 + Solution 2** (Session Tracking with Fingerprinting)

**For banking/critical**: Consider **Solution 3** (Single Session) or strict session limits

