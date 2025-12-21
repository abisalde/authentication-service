package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/abisalde/authentication-service/internal/auth"
	"github.com/abisalde/authentication-service/internal/auth/cookies"
	"github.com/abisalde/authentication-service/internal/middleware"
	"github.com/abisalde/authentication-service/pkg/jwt"
	"github.com/abisalde/authentication-service/pkg/session"
)

func TestSessionCreation_OnLogin(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user with password
	user, err := client.User.Create().
		SetEmail("session@example.com").
		SetFirstName("Test").
		SetLastName("User").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Generate tokens
	tokens, err := cookies.GenerateLoginTokenPair(user.ID)
	if err != nil {
		t.Fatalf("Failed to generate tokens: %v", err)
	}

	// Create session manager
	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Simulate session creation (as done in login handler)
	deviceInfo := &session.DeviceInfo{
		Type:      "Desktop",
		Name:      "Chrome Browser",
		IPAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

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

	err = sessionManager.CreateSession(ctx, sessionInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session was created
	sessions, err := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if err != nil {
		t.Fatalf("Failed to get user sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].DeviceType != "Desktop" {
		t.Errorf("Expected device type Desktop, got %s", sessions[0].DeviceType)
	}
}

func TestMultipleDeviceSessions(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("multidevice@example.com").
		SetFirstName("Multi").
		SetLastName("Device").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Create sessions for multiple devices
	devices := []struct {
		name string
		typ  string
	}{
		{"Chrome Browser", "Desktop"},
		{"iPhone", "Mobile"},
		{"iPad", "Tablet"},
	}

	for _, device := range devices {
		tokens, _ := cookies.GenerateLoginTokenPair(user.ID)
		
		sessionInfo := &session.SessionInfo{
			UserID:     strconv.FormatInt(user.ID, 10),
			DeviceType: device.typ,
			DeviceName: device.name,
			IPAddress:  "192.168.1.1",
			UserAgent:  "Test",
			TokenHash:  session.HashToken(tokens.AccessToken),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
			ExpiresAt:  time.Now().Add(12 * time.Hour),
		}

		err = sessionManager.CreateSession(ctx, sessionInfo)
		if err != nil {
			t.Fatalf("Failed to create session for %s: %v", device.name, err)
		}
	}

	// Verify all sessions were created
	sessions, err := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if err != nil {
		t.Fatalf("Failed to get user sessions: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(sessions))
	}

	// Verify each device type
	deviceTypes := make(map[string]bool)
	for _, sess := range sessions {
		deviceTypes[sess.DeviceType] = true
	}

	if !deviceTypes["Desktop"] || !deviceTypes["Mobile"] || !deviceTypes["Tablet"] {
		t.Error("Not all device types found in sessions")
	}
}

func TestSessionRevocation(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("revoke@example.com").
		SetFirstName("Revoke").
		SetLastName("Test").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Create two sessions
	tokens1, _ := cookies.GenerateLoginTokenPair(user.ID)
	tokens2, _ := cookies.GenerateLoginTokenPair(user.ID)

	sessionInfo1 := &session.SessionInfo{
		UserID:     strconv.FormatInt(user.ID, 10),
		DeviceType: "Desktop",
		DeviceName: "Desktop 1",
		TokenHash:  session.HashToken(tokens1.AccessToken),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(12 * time.Hour),
	}

	sessionInfo2 := &session.SessionInfo{
		UserID:     strconv.FormatInt(user.ID, 10),
		DeviceType: "Mobile",
		DeviceName: "iPhone",
		TokenHash:  session.HashToken(tokens2.AccessToken),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(12 * time.Hour),
	}

	sessionManager.CreateSession(ctx, sessionInfo1)
	sessionManager.CreateSession(ctx, sessionInfo2)

	// Verify 2 sessions exist
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 2 {
		t.Fatalf("Expected 2 sessions, got %d", len(sessions))
	}

	// Revoke one session
	sessionIDToRevoke := sessions[0].SessionID
	err = sessionManager.RevokeSession(ctx, strconv.FormatInt(user.ID, 10), sessionIDToRevoke)
	if err != nil {
		t.Fatalf("Failed to revoke session: %v", err)
	}

	// Verify only 1 session remains
	sessions, _ = sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session after revocation, got %d", len(sessions))
	}
}

func TestLogoutFromAllDevices(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("logoutall@example.com").
		SetFirstName("Logout").
		SetLastName("All").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		tokens, _ := cookies.GenerateLoginTokenPair(user.ID)
		
		sessionInfo := &session.SessionInfo{
			UserID:     strconv.FormatInt(user.ID, 10),
			DeviceType: "Desktop",
			DeviceName: fmt.Sprintf("Device %d", i+1),
			TokenHash:  session.HashToken(tokens.AccessToken),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
			ExpiresAt:  time.Now().Add(12 * time.Hour),
		}

		sessionManager.CreateSession(ctx, sessionInfo)
	}

	// Verify 5 sessions exist
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 5 {
		t.Fatalf("Expected 5 sessions, got %d", len(sessions))
	}

	// Logout from all devices
	err = sessionManager.RevokeAllUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if err != nil {
		t.Fatalf("Failed to revoke all sessions: %v", err)
	}

	// Verify no sessions remain
	sessions, _ = sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after logout from all devices, got %d", len(sessions))
	}
}

func TestMaxConcurrentSessions(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("maxsessions@example.com").
		SetFirstName("Max").
		SetLastName("Sessions").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())
	maxSessions := 3

	// Create 5 sessions (should enforce max of 3)
	for i := 0; i < 5; i++ {
		tokens, _ := cookies.GenerateLoginTokenPair(user.ID)
		
		sessionInfo := &session.SessionInfo{
			UserID:     strconv.FormatInt(user.ID, 10),
			DeviceType: "Desktop",
			DeviceName: fmt.Sprintf("Device %d", i+1),
			TokenHash:  session.HashToken(tokens.AccessToken),
			CreatedAt:  time.Now(),
			LastUsedAt: time.Now(),
			ExpiresAt:  time.Now().Add(12 * time.Hour),
		}

		// Enforce max sessions before creating
		sessionManager.EnforceMaxSessions(ctx, sessionInfo.UserID, maxSessions)
		sessionManager.CreateSession(ctx, sessionInfo)

		// Small delay to ensure time ordering
		time.Sleep(10 * time.Millisecond)
	}

	// Verify only max sessions remain
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) > maxSessions {
		t.Errorf("Expected max %d sessions, got %d", maxSessions, len(sessions))
	}
}

func TestSessionActivityUpdate(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("activity@example.com").
		SetFirstName("Activity").
		SetLastName("Test").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Create session
	tokens, _ := cookies.GenerateLoginTokenPair(user.ID)
	
	sessionInfo := &session.SessionInfo{
		UserID:     strconv.FormatInt(user.ID, 10),
		DeviceType: "Desktop",
		DeviceName: "Test Device",
		TokenHash:  session.HashToken(tokens.AccessToken),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(12 * time.Hour),
	}

	err = sessionManager.CreateSession(ctx, sessionInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Get initial session
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}
	initialLastUsed := sessions[0].LastUsedAt

	// Wait and update activity
	time.Sleep(100 * time.Millisecond)
	err = sessionManager.UpdateSessionActivity(ctx, sessions[0].SessionID)
	if err != nil {
		t.Fatalf("Failed to update session activity: %v", err)
	}

	// Verify last used time was updated
	sessions, _ = sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if !sessions[0].LastUsedAt.After(initialLastUsed) {
		t.Error("LastUsedAt should have been updated")
	}
}

func TestMiddlewareSessionIntegration(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	user, err := client.User.Create().
		SetEmail("middleware@example.com").
		SetFirstName("Middleware").
		SetLastName("Test").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Generate token
	token, err := jwt.GenerateToken(user.ID, jwt.TokenTypeAccess, 10*time.Minute)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create session manually
	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())
	sessionInfo := &session.SessionInfo{
		UserID:     strconv.FormatInt(user.ID, 10),
		DeviceType: "Desktop",
		DeviceName: "Test Device",
		TokenHash:  session.HashToken(token),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	sessionManager.CreateSession(ctx, sessionInfo)

	// Create test request with token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Create middleware
	authMiddleware := middleware.AuthMiddleware(client, authService)

	// Handler to check if session info is in context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionInfo := r.Context().Value(auth.SessionInfoKey)
		if sessionInfo == nil {
			t.Error("Session info not found in context")
		} else {
			sess, ok := sessionInfo.(*session.SessionInfo)
			if !ok {
				t.Error("Session info is not of correct type")
			} else if sess.DeviceType != "Desktop" {
				t.Errorf("Expected device type Desktop, got %s", sess.DeviceType)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware and serve
	authMiddleware(handler).ServeHTTP(rr, req)

	// Verify session activity was updated
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}
	
	// LastUsedAt should be recent (within last second)
	timeSinceLastUsed := time.Since(sessions[0].LastUsedAt)
	if timeSinceLastUsed > 2*time.Second {
		t.Errorf("Session activity was not updated, time since last used: %v", timeSinceLastUsed)
	}
}

func TestSessionExpiration(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("expiration@example.com").
		SetFirstName("Expiration").
		SetLastName("Test").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Create session with short expiration
	tokens, _ := cookies.GenerateLoginTokenPair(user.ID)
	
	sessionInfo := &session.SessionInfo{
		UserID:     strconv.FormatInt(user.ID, 10),
		DeviceType: "Desktop",
		DeviceName: "Test Device",
		TokenHash:  session.HashToken(tokens.AccessToken),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(1 * time.Second),
	}

	err = sessionManager.CreateSession(ctx, sessionInfo)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session exists
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Session should be automatically removed by Redis
	sessions, _ = sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after expiration, got %d", len(sessions))
	}
}

func TestDeviceDetection(t *testing.T) {
	tests := []struct {
		name          string
		userAgent     string
		expectedType  string
		expectedName  string
	}{
		{
			name:         "Chrome Desktop",
			userAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expectedType: "Desktop",
			expectedName: "Chrome Browser on Windows",
		},
		{
			name:         "iPhone",
			userAgent:    "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			expectedType: "Mobile",
			expectedName: "iPhone",
		},
		{
			name:         "iPad",
			userAgent:    "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			expectedType: "Tablet",
			expectedName: "iPad",
		},
		{
			name:         "Android Mobile",
			userAgent:    "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			expectedType: "Mobile",
			expectedName: "Android Device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("User-Agent", tt.userAgent)
			req.Header.Set("X-Real-IP", "192.168.1.1")

			deviceInfo := session.ExtractDeviceInfo(req)

			if deviceInfo.Type != tt.expectedType {
				t.Errorf("Expected device type %s, got %s", tt.expectedType, deviceInfo.Type)
			}

			if deviceInfo.Name != tt.expectedName {
				t.Errorf("Expected device name %s, got %s", tt.expectedName, deviceInfo.Name)
			}

			if deviceInfo.IPAddress != "192.168.1.1" {
				t.Errorf("Expected IP 192.168.1.1, got %s", deviceInfo.IPAddress)
			}
		})
	}
}

func TestConcurrentSessionOperations(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("concurrent@example.com").
		SetFirstName("Concurrent").
		SetLastName("Test").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())

	// Concurrently create sessions
	concurrency := 10
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			tokens, _ := cookies.GenerateLoginTokenPair(user.ID)
			
			sessionInfo := &session.SessionInfo{
				UserID:     strconv.FormatInt(user.ID, 10),
				DeviceType: "Desktop",
				DeviceName: fmt.Sprintf("Device %d", index),
				TokenHash:  session.HashToken(tokens.AccessToken),
				CreatedAt:  time.Now(),
				LastUsedAt: time.Now(),
				ExpiresAt:  time.Now().Add(12 * time.Hour),
			}

			err := sessionManager.CreateSession(ctx, sessionInfo)
			results <- err
		}(i)
	}

	// Wait for all goroutines
	errorCount := 0
	for i := 0; i < concurrency; i++ {
		if err := <-results; err != nil {
			errorCount++
			t.Logf("Error creating session: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors, got %d errors", errorCount)
	}

	// Verify sessions were created
	sessions, _ := sessionManager.GetUserSessions(ctx, strconv.FormatInt(user.ID, 10))
	if len(sessions) != concurrency {
		t.Errorf("Expected %d sessions, got %d", concurrency, len(sessions))
	}
}

func TestGraphQLAuthDirective_WithSession(t *testing.T) {
	authService, client, cleanup := setupTestAuthService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	user, err := client.User.Create().
		SetEmail("graphql@example.com").
		SetFirstName("GraphQL").
		SetLastName("Test").
		SetPasswordHash("$2a$10$test.hash.value").
		SetIsEmailVerified(true).
		SetRole("USER").
		Save(ctx)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Generate token
	token, err := jwt.GenerateToken(user.ID, jwt.TokenTypeAccess, 10*time.Minute)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create session
	sessionManager := session.NewSessionManager(authService.GetCache().RawClient())
	sessionInfo := &session.SessionInfo{
		UserID:     strconv.FormatInt(user.ID, 10),
		DeviceType: "Desktop",
		DeviceName: "Test Device",
		TokenHash:  session.HashToken(token),
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}
	sessionManager.CreateSession(ctx, sessionInfo)

	// Add user to context (simulating middleware)
	ctx = context.WithValue(ctx, auth.CurrentUserKey, user)
	ctx = context.WithValue(ctx, auth.SessionInfoKey, sessionInfo)

	// Verify session exists in context
	sessionFromCtx := ctx.Value(auth.SessionInfoKey)
	if sessionFromCtx == nil {
		t.Error("Session not found in context")
	}

	sess, ok := sessionFromCtx.(*session.SessionInfo)
	if !ok {
		t.Error("Session is not of correct type")
	}

	if sess.DeviceType != "Desktop" {
		t.Errorf("Expected device type Desktop, got %s", sess.DeviceType)
	}

	// Verify user is authenticated
	currentUser := auth.GetCurrentUser(ctx)
	if currentUser == nil {
		t.Error("Expected user in context")
	}

	if currentUser.ID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, currentUser.ID)
	}
}
