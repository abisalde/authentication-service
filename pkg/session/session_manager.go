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
	SessionID  string    `json:"session_id"`
	UserID     string    `json:"user_id"`
	DeviceType string    `json:"device_type"` // Desktop, Mobile, Tablet
	DeviceName string    `json:"device_name"` // "Chrome on Windows", "iPhone 13"
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	TokenHash  string    `json:"token_hash"` // Hash of access token
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	ExpiresAt  time.Time `json:"expires_at"`
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
	if ttl <= 0 {
		return fmt.Errorf("session already expired")
	}

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
	if ttl <= 0 {
		// Session expired
		return fmt.Errorf("session expired")
	}

	return sm.redisClient.Set(ctx, sessionKey, sessionData, ttl).Err()
}

// RevokeSession revokes a specific session
func (sm *SessionManager) RevokeSession(ctx context.Context, userID, sessionID string) error {
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

	// Clean up the user sessions set
	userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
	return sm.redisClient.Del(ctx, userSessionsKey).Err()
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

// CountUserSessions returns the number of active sessions for a user
func (sm *SessionManager) CountUserSessions(ctx context.Context, userID string) (int, error) {
	userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
	count, err := sm.redisClient.SCard(ctx, userSessionsKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count user sessions: %w", err)
	}
	return int(count), nil
}

// EnforceMaxSessions ensures user doesn't exceed max concurrent sessions
// Revokes oldest sessions if limit is exceeded
func (sm *SessionManager) EnforceMaxSessions(ctx context.Context, userID string, maxSessions int) error {
	if maxSessions <= 0 {
		return nil // No limit
	}

	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	if len(sessions) >= maxSessions {
		// Sort by creation time (oldest first)
		sortedSessions := make([]*SessionInfo, len(sessions))
		copy(sortedSessions, sessions)
		
		// Simple bubble sort by CreatedAt
		for i := 0; i < len(sortedSessions)-1; i++ {
			for j := 0; j < len(sortedSessions)-i-1; j++ {
				if sortedSessions[j].CreatedAt.After(sortedSessions[j+1].CreatedAt) {
					sortedSessions[j], sortedSessions[j+1] = sortedSessions[j+1], sortedSessions[j]
				}
			}
		}

		// Remove oldest sessions to make room
		toRemove := len(sortedSessions) - maxSessions + 1
		for i := 0; i < toRemove && i < len(sortedSessions); i++ {
			if err := sm.RevokeSession(ctx, userID, sortedSessions[i].SessionID); err != nil {
				fmt.Printf("Failed to revoke old session %s: %v\n", sortedSessions[i].SessionID, err)
			}
		}
	}

	return nil
}

// generateSessionID generates a unique session ID
func generateSessionID(userID, tokenHash string) string {
	data := fmt.Sprintf("%s:%s:%d", userID, tokenHash, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// HashToken creates a hash of a token for storage
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
