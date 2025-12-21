package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/abisalde/authentication-service/pkg/session"
	"github.com/redis/go-redis/v9"
)

// This example demonstrates multi-device session management
// It shows how to:
// 1. Create sessions on login with device tracking
// 2. View all active sessions
// 3. Revoke specific sessions
// 4. Logout from all devices

func main() {
	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Initialize session manager
	sessionManager := session.NewSessionManager(redisClient)

	// Setup HTTP routes
	http.HandleFunc("/login", handleLogin(sessionManager))
	http.HandleFunc("/sessions", handleGetSessions(sessionManager))
	http.HandleFunc("/sessions/revoke", handleRevokeSession(sessionManager))
	http.HandleFunc("/sessions/revoke-others", handleRevokeOtherSessions(sessionManager))
	http.HandleFunc("/logout-all", handleLogoutAll(sessionManager))

	log.Println("Session management example running on :8082")
	log.Println("Endpoints:")
	log.Println("  POST /login - Login and create session")
	log.Println("  GET /sessions?user_id=123 - View all active sessions")
	log.Println("  POST /sessions/revoke - Revoke specific session")
	log.Println("  POST /sessions/revoke-others - Logout from other devices")
	log.Println("  POST /logout-all - Logout from all devices")
	
	http.ListenAndServe(":8082", nil)
}

// handleLogin simulates login and creates a session
func handleLogin(sm *session.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// In real implementation, validate credentials first
		var req struct {
			UserID string `json:"user_id"`
			Token  string `json:"token"` // Simulated JWT token
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Extract device information
		deviceInfo := session.ExtractDeviceInfo(r)

		// Create session
		sessionInfo := &session.SessionInfo{
			UserID:     req.UserID,
			DeviceType: deviceInfo.Type,
			DeviceName: deviceInfo.Name,
			IPAddress:  deviceInfo.IPAddress,
			UserAgent:  deviceInfo.UserAgent,
			TokenHash:  session.HashToken(req.Token),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
			ExpiresAt:  time.Now().Add(12 * time.Hour),
		}

		ctx := r.Context()
		
		// Optional: Enforce max sessions (e.g., max 5 devices)
		if err := sm.EnforceMaxSessions(ctx, req.UserID, 5); err != nil {
			log.Printf("Failed to enforce max sessions: %v", err)
		}

		if err := sm.CreateSession(ctx, sessionInfo); err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"session_id": sessionInfo.SessionID,
			"message":    fmt.Sprintf("Logged in from %s", deviceInfo.Name),
		})
	}
}

// handleGetSessions returns all active sessions for a user
func handleGetSessions(sm *session.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id required", http.StatusBadRequest)
			return
		}

		sessions, err := sm.GetUserSessions(r.Context(), userID)
		if err != nil {
			http.Error(w, "Failed to get sessions", http.StatusInternalServerError)
			return
		}

		// Format response
		type SessionResponse struct {
			SessionID  string `json:"session_id"`
			DeviceType string `json:"device_type"`
			DeviceName string `json:"device_name"`
			IPAddress  string `json:"ip_address"`
			CreatedAt  string `json:"created_at"`
			LastUsedAt string `json:"last_used_at"`
			IsCurrent  bool   `json:"is_current"`
		}

		currentTokenHash := r.URL.Query().Get("current_token_hash")
		
		response := make([]SessionResponse, 0, len(sessions))
		for _, s := range sessions {
			response = append(response, SessionResponse{
				SessionID:  s.SessionID,
				DeviceType: s.DeviceType,
				DeviceName: s.DeviceName,
				IPAddress:  s.IPAddress,
				CreatedAt:  s.CreatedAt.Format(time.RFC3339),
				LastUsedAt: s.LastUsedAt.Format(time.RFC3339),
				IsCurrent:  s.TokenHash == currentTokenHash,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": response,
			"count":    len(response),
		})
	}
}

// handleRevokeSession revokes a specific session
func handleRevokeSession(sm *session.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			UserID    string `json:"user_id"`
			SessionID string `json:"session_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := sm.RevokeSession(r.Context(), req.UserID, req.SessionID); err != nil {
			http.Error(w, "Failed to revoke session", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Session revoked successfully",
		})
	}
}

// handleRevokeOtherSessions revokes all sessions except the current one
func handleRevokeOtherSessions(sm *session.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			UserID           string `json:"user_id"`
			CurrentSessionID string `json:"current_session_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := sm.RevokeOtherSessions(r.Context(), req.UserID, req.CurrentSessionID); err != nil {
			http.Error(w, "Failed to revoke sessions", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "All other sessions revoked successfully",
		})
	}
}

// handleLogoutAll revokes all sessions (logout from all devices)
func handleLogoutAll(sm *session.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			UserID string `json:"user_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if err := sm.RevokeAllUserSessions(r.Context(), req.UserID); err != nil {
			http.Error(w, "Failed to logout from all devices", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Logged out from all devices",
		})
	}
}
